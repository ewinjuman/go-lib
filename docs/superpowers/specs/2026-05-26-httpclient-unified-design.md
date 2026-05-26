# Unified `httpclient` Package — Design Spec

**Date:** 2026-05-26  
**Status:** Approved  
**Branch:** v2  

---

## Problem Statement

The repository currently maintains two HTTP client packages (`httpclient` using resty, `httpstd` using stdlib) with identical public APIs. This creates:

1. Dual-maintenance burden — every API change must be mirrored in both packages.
2. External dependency on `go-resty/resty` that can be eliminated.
3. Missing features: no retry/backoff, no middleware hooks, no response headers, no SSE, no cookie jar, no proxy, no GraphQL sugar, limited content types.
4. No shared `Client` instance for reusing config (base URL, auth, default headers).

## Goal

Replace both packages with a single unified `httpclient` package that:

- Uses **only `net/http` stdlib** — zero external HTTP dependencies.
- Provides a **`Client` instance** for shared config with a **fluent builder** per request.
- Supports all common HTTP request/response patterns.
- Adds a composable **middleware chain** (logging, retry, circuit breaker).
- Maintains **backward-compatible package-level shortcuts** so existing call sites need no changes.

---

## Architecture

### Core Interfaces

```go
// Doer is the minimal interface for sending an HTTP request.
// net/http.Client satisfies this structurally.
type Doer interface {
    Do(*http.Request) (*http.Response, error)
}

// DoerFunc allows plain functions to implement Doer.
type DoerFunc func(*http.Request) (*http.Response, error)
func (f DoerFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

// Middleware wraps a Doer to add behaviour (logging, retry, circuit breaking, etc.)
type Middleware func(next Doer) Doer
```

### Transport Stack

Middleware is composed into a stack at `Client` construction time. The first middleware registered is the outermost layer — it sees the request first and the response last.

```
Request  →  [Logger] → [Retry] → [CircuitBreaker] → net/http.Client
Response ←  [Logger] ← [Retry] ← [CircuitBreaker] ← net/http.Client
```

```go
// Apply composes all middleware onto a base Doer (right to left).
func Apply(base Doer, middlewares ...Middleware) Doer {
    for i := len(middlewares) - 1; i >= 0; i-- {
        base = middlewares[i](base)
    }
    return base
}
```

### Component Map

```
httpclient.New(opts...)
       │
       ▼
   Client
   ├── baseURL            string
   ├── defaultHeaders     http.Header
   ├── defaultTimeout     time.Duration
   ├── cookieJar          http.CookieJar
   ├── middlewares        []Middleware
   └── doer               Doer   ← composed stack
       │
       └── .Post(path) / .Get(path) / ... → RequestBuilder
                                                │
                                                ▼
                                             Execute() / ExecuteSSE()
                                                │
                                                ▼
                                             Response
                                             ├── StatusCode int
                                             ├── Body       []byte
                                             ├── Headers    http.Header
                                             └── Error      error
```

---

## Package File Structure

```
httpclient/
  client.go          — Client struct, New(), ClientOption funcs
  request.go         — RequestBuilder struct, all With* methods
  execute.go         — Execute(), ExecuteSSE(), body building, URL prep
  response.go        — Response struct, Consume(), ConsumeXML(), ConsumeText(),
                       IsSuccess(), IsError(), SaveToFile(), Raw()
  middleware.go      — Middleware type, Apply(), DoerFunc, middleware keys
  retry.go           — RetryMiddleware, BackoffFunc, FixedBackoff,
                       ExponentialBackoff, ExponentialWithJitter
  circuit_breaker.go — CircuitBreakerMiddleware (ported from current CB logic)
  sse.go             — ExecuteSSE(), SSEEvent struct, SSE line parser
```

---

## Client

### Construction

```go
client := httpclient.New(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithDefaultTimeout(10 * time.Second),
    httpclient.WithDefaultHeaders(map[string]string{"X-App": "myapp"}),
    httpclient.WithMiddleware(httpclient.LoggingMiddleware(log)),
    httpclient.WithMiddleware(httpclient.RetryMiddleware(retryConfig)),
    httpclient.WithMiddleware(httpclient.CircuitBreakerMiddleware(cbConfig)),
)
```

### ClientOption functions

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Prefix for all relative paths |
| `WithDefaultTimeout(d)` | Applied when request has no per-request timeout |
| `WithDefaultHeaders(map)` | Merged into every request |
| `WithDefaultBearer(fn func() string)` | Token provider called per request |
| `WithDefaultBasicAuth(user, pass)` | Applied to every request |
| `WithMiddleware(m)` | Appends middleware to the stack |
| `WithCookieJar(jar)` | Enables cookie management |
| `WithSkipTLS()` | InsecureSkipVerify = true |
| `WithTLSConfig(cfg)` | Custom tls.Config |
| `WithProxy(url)` | HTTP proxy |
| `WithTransport(t)` | Full net/http.RoundTripper override |

### Package-level shortcuts (backward compatible)

```go
// These use a default global Client with no base URL and no middleware.
httpclient.Post("https://api.example.com/users").WithBody(x).Execute()
httpclient.Get(url).Execute()
// etc.
```

---

## Request Builder

`RequestBuilder` is created by `client.Post(path)`, `client.Get(path)`, etc.  
It holds per-request overrides on top of the Client's shared config.

### With* methods

| Method | Description |
|--------|-------------|
| `WithBody(v)` | JSON body (default content type) |
| `WithForm(v)` | `application/x-www-form-urlencoded` |
| `WithMultipart(parts...)` | Multipart/form-data with fields and files |
| `WithRawBody(b []byte, ct string)` | Raw bytes with explicit Content-Type |
| `WithGraphQL(query, vars)` | JSON body `{query, variables}` sugar |
| `WithQueryParams(map)` | Append query string |
| `WithPathParam(key, val)` | Replace `:key` in URL |
| `WithHeaders(map)` | Per-request headers (merged with defaults) |
| `WithBearer(token)` | Override Authorization: Bearer |
| `WithBasicAuth(user, pass)` | Override Authorization: Basic |
| `WithCookie(name, val)` | Add a single cookie |
| `WithTimeout(d)` | Override default timeout |
| `WithOutput(w io.Writer)` | Stream response body to writer |
| `WithoutMiddleware(keys...)` | Skip named middlewares for this request |
| `WithSuccessCodes(codes)` | Override success status codes |
| `WithRequestID(id)` | Set X-Request-ID header |
| `WithDebug(bool)` | Enable request/response logging |
| `WithSkipTLS()` | InsecureSkipVerify for this request |
| `WithContext(ctx)` | Set or override context |

### Multipart helpers

```go
httpclient.Field(key, value string) MultipartPart
httpclient.FileFromReader(field, filename string, r io.Reader) MultipartPart
httpclient.FileFromPath(field, path string) MultipartPart
```

### Per-request middleware skip

`WithoutMiddleware(keys...)` accepts the exported key constants:

```go
httpclient.CircuitBreakerKey  // "circuit_breaker"
httpclient.RetryKey           // "retry"
httpclient.LoggingKey         // "logging"
```

`Execute()` rebuilds the middleware stack excluding the skipped keys. Rebuild cost is O(n) middleware count — negligible.

---

## Response

```go
type Response struct {
    StatusCode   int
    Body         []byte         // nil if WithOutput was used (streaming)
    Headers      http.Header    // response headers
    Error        error
    SuccessCodes []int
    raw          *http.Response // closed after Execute(); use Raw() before that
}
```

### Methods

| Method | Description |
|--------|-------------|
| `Consume(v)` | JSON unmarshal into v; returns error if status not success |
| `ConsumeXML(v)` | XML unmarshal into v |
| `ConsumeText()` | Return body as string |
| `IsSuccess()` | true if no error and status in SuccessCodes |
| `IsError()` | (bool, error) — true if error or status not in SuccessCodes |
| `HttpCode()` | Returns StatusCode |
| `SaveToFile(path)` | Write Body to file at path |
| `Raw()` | Return underlying *http.Response (nil after body is read) |

---

## Middleware

### LoggingMiddleware

Logs request before send and response (status + duration) after return.  
Uses the existing `logger.Writer` interface for masking and redaction support.

```go
httpclient.LoggingMiddleware(writer logger.Writer) Middleware
```

### RetryMiddleware

```go
type RetryConfig struct {
    MaxAttempts int
    Backoff     BackoffFunc                                // delay per attempt
    RetryOn     func(resp *http.Response, err error) bool // when to retry
}

type BackoffFunc func(attempt int) time.Duration

// Built-in backoff strategies
httpclient.FixedBackoff(d time.Duration) BackoffFunc
httpclient.ExponentialBackoff(base time.Duration, multiplier float64) BackoffFunc
httpclient.ExponentialWithJitter(base time.Duration, multiplier float64) BackoffFunc

// Built-in retry conditions
httpclient.RetryOnError                        // network errors only
httpclient.RetryOnStatus(codes ...int)         // specific HTTP status codes
httpclient.RetryOnAny                          // network errors + 5xx
```

Retry respects `context.Done()` — cancellation stops retry loop immediately.

### CircuitBreakerMiddleware

Ported from current `circuit_breaker.go`. State remains global per host (`sync.Map` keyed by `scheme://host`).

```go
type CBConfig struct {
    MaxFailures  int
    Timeout      time.Duration
    HalfOpenMax  int
}

httpclient.CircuitBreakerMiddleware(cfg CBConfig) Middleware
```

5xx responses record failure; 4xx and 2xx record success (same logic as current).

### Custom Middleware

```go
func TenantMiddleware(tenantID string) httpclient.Middleware {
    return func(next httpclient.Doer) httpclient.Doer {
        return httpclient.DoerFunc(func(req *http.Request) (*http.Response, error) {
            req.Header.Set("X-Tenant-ID", tenantID)
            return next.Do(req)
        })
    }
}
```

---

## SSE (Server-Sent Events)

`ExecuteSSE` replaces `Execute` for `text/event-stream` endpoints.

```go
type SSEEvent struct {
    ID    string
    Event string // defaults to "message" if no event field
    Data  string
    Retry int    // reconnect hint in ms (from server)
}

err := client.Get("/events").ExecuteSSE(func(event SSEEvent) error {
    // process event
    // return non-nil error to stop receiving
    return nil
})
```

Implementation: reads response body line-by-line with `bufio.Scanner`, parses `field: value` pairs per the SSE spec (RFC 8895), dispatches complete events to the callback. Respects `context.Context` cancellation.

---

## Body Building

`buildBody()` in `execute.go` uses an internal `bodyMode` enum:

```go
type bodyMode int
const (
    bodyModeNone      bodyMode = iota
    bodyModeJSON
    bodyModeForm
    bodyModeMultipart
    bodyModeRaw
)
```

`bufPool` (`sync.Pool[*bytes.Buffer]`) is retained for JSON encoding — same zero-alloc path as current `httpstd`.

For multipart: `buildBody` writes fields and files into a `multipart.Writer`, returns the buffer as `io.Reader` and sets the `Content-Type` override to include the boundary.

---

## Migration Guide

### Breaking changes

| Old | New |
|-----|-----|
| `import ".../httpstd"` | `import ".../httpclient"` |
| `import ".../httpclient"` (resty) | same import, new impl |
| `WithFile([]MultipartData{...})` | `WithMultipart(Field(...), FileFromReader(...))` |
| `DisableCircuitBreaker: true` in Request | `WithoutMiddleware(CircuitBreakerKey)` |

### Non-breaking (no changes needed)

- All package-level shortcuts (`httpclient.Post`, `.Get`, etc.) remain.
- `WithBody`, `WithHeaders`, `WithQueryParams`, `WithPathParam`, `WithBearer`, `WithBasicAuth`, `WithTimeout`, `WithSkipTLS`, `WithContext`, `WithOutput`, `WithSuccessCodes`, `WithRequestID`, `WithDebug` — identical signatures.
- `Response.Consume`, `IsSuccess`, `IsError`, `HttpCode`, `SaveToFile` — identical.

---

## Feature Coverage Summary

| Feature | Before | After |
|---------|--------|-------|
| JSON request/response | ✅ | ✅ |
| Form URL-encoded | ✅ | ✅ |
| Multipart file upload | ✅ | ✅ improved API |
| Raw body (XML, protobuf) | ❌ | ✅ |
| GraphQL sugar | ❌ | ✅ |
| SSE / event-stream | ❌ | ✅ `ExecuteSSE()` |
| Response headers | ❌ | ✅ |
| Cookie jar | ❌ | ✅ |
| Retry + backoff | ❌ | ✅ middleware |
| Circuit breaker | ✅ (flag) | ✅ middleware |
| Custom middleware | ❌ | ✅ |
| Per-request middleware skip | ❌ | ✅ |
| Dynamic token provider | ❌ | ✅ |
| Proxy support | ❌ | ✅ |
| stdlib only (no resty) | ⚠️ partial | ✅ |
| Single package | ❌ (2 packages) | ✅ |

---

## Testing Strategy

- Unit tests per middleware (retry, circuit breaker) using `httptest.Server`.
- `RequestBuilder` body building tested in isolation (no network).
- SSE parser unit-tested against spec-conformant event streams.
- Integration tests: end-to-end through `httptest.NewServer`.
- Benchmark: compare new unified package against old `httpstd` on parallel JSON POST load.
- Existing `httpclient` and `httpstd` tests migrated and adapted.
