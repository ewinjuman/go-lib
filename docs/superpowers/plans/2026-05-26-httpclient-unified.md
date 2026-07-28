# Unified `httpclient` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `httpclient` (resty-based) and `httpstd` with a single stdlib-only `httpclient` package featuring a composable middleware chain, `Client` instance with shared config, and full HTTP feature coverage.

**Architecture:** `Client` holds shared config and a `[]Middleware` list. Each `RequestBuilder` is created from `Client` and carries per-request overrides. `Execute()` composes the middleware stack on each call (skipping any keys from `WithoutMiddleware`), builds `*http.Request`, and returns a `Response` with status, body, headers, and error. Retry and circuit breaker are middleware — not flags on the request struct.

**Tech Stack:** `net/http`, `bufio`, `bytes`, `encoding/json`, `encoding/xml`, `mime/multipart`, `net/url`, `sync`, `crypto/tls` (all stdlib). `github.com/google/uuid` (already in go.mod) retained for request IDs. No resty.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `httpclient/middleware.go` | **Replace** | `Doer`, `DoerFunc`, `Middleware`, `NewMiddleware`, `Apply`, key constants |
| `httpclient/response.go` | **Replace** | `Response`, `Consume`, `ConsumeXML`, `ConsumeText`, `IsSuccess`, `IsError`, `HttpCode`, `SaveToFile`, `Raw` |
| `httpclient/client.go` | **Create** (replaces `http_client.go`) | `Client`, `New()`, all `ClientOption` funcs, `buildHTTPClient()`, builder shortcut methods |
| `httpclient/request.go` | **Replace** `http.go` | `Method`, `bodyMode`, `MultipartPart`, `Field`/`FileFromReader`/`FileFromPath`, `RequestBuilder`, all `With*` methods |
| `httpclient/execute.go` | **Replace** | `buildBody()`, `buildURL()`, `applyDefaults()`, `Execute()`, `logResponse()` |
| `httpclient/retry.go` | **Create** | `RetryConfig`, `BackoffFunc`, `FixedBackoff`, `ExponentialBackoff`, `ExponentialWithJitter`, `RetryMiddleware`, `RetryOn*` |
| `httpclient/circuit_breaker.go` | **Extend** | Port existing CB logic; append `CircuitBreakerMiddleware() Middleware` |
| `httpclient/sse.go` | **Create** | `SSEEvent`, `ExecuteSSE()`, SSE line parser |
| `httpclient/http.go` | **Create** (replaces old `http.go` + `http_client.go`) | `defaultClient`, package-level `Post`/`Get`/etc., `WithQueryParam` alias |
| `httpclient/http_client.go` | **Delete** | Replaced by `client.go` |
| `httpclient/middleware_test.go` | **Create** | Tests for `Apply`, `DoerFunc`, skip logic |
| `httpclient/response_test.go` | **Create** | Tests for all `Response` methods |
| `httpclient/client_test.go` | **Create** | Tests for `New()`, options, builder creation |
| `httpclient/request_test.go` | **Create** | Tests for all `With*` methods |
| `httpclient/execute_test.go` | **Replace** | Integration tests via `httptest.Server` |
| `httpclient/download_test.go` | **Delete** | Absorbed into `execute_test.go` |
| `httpclient/retry_test.go` | **Create** | Tests for backoff strategies and retry logic |
| `httpclient/circuit_breaker_test.go` | **Create** | CB state machine tests |
| `httpclient/sse_test.go` | **Create** | SSE parser and `ExecuteSSE` tests |
| `httpstd/` | **Delete entire package** | Replaced by unified `httpclient` |
| `go.mod` / `go.sum` | **Modify** | Remove `go-resty/resty` via `go mod tidy` |
| `CLAUDE.md` | **Update** | Rewrite HTTP Client Design section |
| `README.MD` | **Update** | Update HTTP client examples |

---

## Task 1: Middleware Core

**Files:**
- Create: `httpclient/middleware.go`
- Create: `httpclient/middleware_test.go`

- [ ] **Step 1: Write failing tests**

```go
// httpclient/middleware_test.go
package httpclient

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseDoer(status int) Doer {
	return DoerFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: http.NoBody}, nil
	})
}

func TestDoerFunc_implementsDoer(t *testing.T) {
	df := DoerFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 204, Body: http.NoBody}, nil
	})
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := df.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 204, resp.StatusCode)
}

func TestApply_firstMiddlewareIsOutermost(t *testing.T) {
	var order []string
	m1 := NewMiddleware("m1", func(next Doer) Doer {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			order = append(order, "m1-req")
			resp, err := next.Do(r)
			order = append(order, "m1-resp")
			return resp, err
		})
	})
	m2 := NewMiddleware("m2", func(next Doer) Doer {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			order = append(order, "m2-req")
			resp, err := next.Do(r)
			order = append(order, "m2-resp")
			return resp, err
		})
	})
	composed := Apply(baseDoer(200), []Middleware{m1, m2}, nil)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, err := composed.Do(req)
	require.NoError(t, err)
	assert.Equal(t, []string{"m1-req", "m2-req", "m2-resp", "m1-resp"}, order)
}

func TestApply_skipsMiddlewareByKey(t *testing.T) {
	called := false
	m := NewMiddleware("skip-me", func(next Doer) Doer {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			called = true
			return next.Do(r)
		})
	})
	composed := Apply(baseDoer(200), []Middleware{m}, map[string]bool{"skip-me": true})
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, _ = composed.Do(req)
	assert.False(t, called)
}

func TestApply_nilSkipMap_callsAllMiddleware(t *testing.T) {
	called := false
	m := NewMiddleware("m", func(next Doer) Doer {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			called = true
			return next.Do(r)
		})
	})
	composed := Apply(baseDoer(200), []Middleware{m}, nil)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, _ = composed.Do(req)
	assert.True(t, called)
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```
go test ./httpclient/... -run "TestDoerFunc|TestApply" -v
```
Expected: `undefined: Doer` compile error

- [ ] **Step 3: Write implementation**

```go
// httpclient/middleware.go
package httpclient

import (
	"net/http"
	"time"

	"github.com/ewinjuman/go-lib/v2/logger"
)

// Key constants for built-in middleware — pass to WithoutMiddleware to skip per request.
const (
	LoggingKey        = "logging"
	RetryKey          = "retry"
	CircuitBreakerKey = "circuit_breaker"
)

// Doer sends an HTTP request and returns a response.
// *http.Client satisfies this interface structurally.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// DoerFunc is a function that implements Doer.
type DoerFunc func(*http.Request) (*http.Response, error)

func (f DoerFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

// Middleware is a named wrapper that adds behaviour around a Doer.
// The key identifies the middleware for per-request skipping via WithoutMiddleware.
type Middleware struct {
	key string
	fn  func(Doer) Doer
}

// NewMiddleware creates a custom Middleware with the given key and wrapping function.
func NewMiddleware(key string, fn func(Doer) Doer) Middleware {
	return Middleware{key: key, fn: fn}
}

// Apply composes middlewares onto base right-to-left.
// The first middleware in the slice is outermost: first called on request, last on response.
// Middlewares whose key appears in skip are excluded from the stack.
func Apply(base Doer, middlewares []Middleware, skip map[string]bool) Doer {
	d := base
	for i := len(middlewares) - 1; i >= 0; i-- {
		m := middlewares[i]
		if skip[m.key] {
			continue
		}
		d = m.fn(d)
	}
	return d
}

// LoggingMiddleware returns a Middleware that logs the request before sending
// and the response (status + duration) after receiving.
// Uses logger.Writer so masking and redaction rules apply.
func LoggingMiddleware(w logger.Writer) Middleware {
	return NewMiddleware(LoggingKey, func(next Doer) Doer {
		return DoerFunc(func(req *http.Request) (*http.Response, error) {
			start := time.Now()
			w.Print(req.Context(), "http_request", req.Method, req.URL.String(), nil, req.Header, nil)

			resp, err := next.Do(req)
			elapsed := time.Since(start)

			if err != nil {
				w.Print(req.Context(), "http_response", req.Method, req.URL.String(), 0, nil, http.Header{}, elapsed, err)
				return nil, err
			}
			w.Print(req.Context(), "http_response", req.Method, req.URL.String(), resp.StatusCode, nil, resp.Header, elapsed, nil)
			return resp, nil
		})
	})
}
```

- [ ] **Step 4: Run tests — expect all pass**

```
go test ./httpclient/... -run "TestDoerFunc|TestApply" -v
```
Expected: 4 tests PASS

- [ ] **Step 5: Commit**

```
git add httpclient/middleware.go httpclient/middleware_test.go
git commit -m "feat(httpclient): add Doer/Middleware core with Apply, key constants, LoggingMiddleware"
```

---

## Task 2: Response

**Files:**
- Replace: `httpclient/response.go`
- Create: `httpclient/response_test.go`

- [ ] **Step 1: Write failing tests**

```go
// httpclient/response_test.go
package httpclient

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jsonBody(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestResponse_IsSuccess(t *testing.T) {
	assert.True(t, (&Response{StatusCode: 200, SuccessCodes: []int{200}}).IsSuccess())
	assert.False(t, (&Response{StatusCode: 404, SuccessCodes: []int{200}}).IsSuccess())
	assert.False(t, (&Response{StatusCode: 200, Error: errors.New("e"), SuccessCodes: []int{200}}).IsSuccess())
}

func TestResponse_IsError(t *testing.T) {
	isErr, err := (&Response{StatusCode: 200, SuccessCodes: []int{200}}).IsError()
	assert.False(t, isErr)
	assert.NoError(t, err)

	isErr, err = (&Response{StatusCode: 500, SuccessCodes: []int{200}}).IsError()
	assert.True(t, isErr)
	assert.Error(t, err)
}

func TestResponse_Consume_success(t *testing.T) {
	type result struct{ Name string }
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: jsonBody(result{"alice"})}
	var got result
	require.NoError(t, r.Consume(&got))
	assert.Equal(t, "alice", got.Name)
}

func TestResponse_Consume_errorPropagated(t *testing.T) {
	r := &Response{Error: errors.New("network error"), SuccessCodes: []int{200}}
	assert.Error(t, r.Consume(nil))
}

func TestResponse_Consume_nonSuccessStatus(t *testing.T) {
	r := &Response{StatusCode: 404, SuccessCodes: []int{200}, Body: []byte(`{"error":"not found"}`)}
	assert.Error(t, r.Consume(nil))
}

func TestResponse_Consume_nilBody_returnsErrEmpty(t *testing.T) {
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: nil}
	assert.ErrorIs(t, r.Consume(nil), ErrEmptyResponseBody)
}

func TestResponse_ConsumeXML(t *testing.T) {
	type root struct {
		Value string `xml:"value"`
	}
	xmlBytes := []byte(`<root><value>hello</value></root>`)
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: xmlBytes}
	var got root
	require.NoError(t, r.ConsumeXML(&got))
	assert.Equal(t, "hello", got.Value)
}

func TestResponse_ConsumeText(t *testing.T) {
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: []byte("plain text")}
	text, err := r.ConsumeText()
	require.NoError(t, err)
	assert.Equal(t, "plain text", text)
}

func TestResponse_SaveToFile(t *testing.T) {
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: []byte("file content")}
	path := filepath.Join(t.TempDir(), "out.txt")
	require.NoError(t, r.SaveToFile(path))
	got, _ := os.ReadFile(path)
	assert.Equal(t, "file content", string(got))
}

func TestResponse_Headers_accessible(t *testing.T) {
	h := http.Header{"X-Custom": []string{"value"}}
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Headers: h}
	assert.Equal(t, "value", r.Headers.Get("X-Custom"))
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```
go test ./httpclient/... -run "TestResponse" -v
```
Expected: compile error — `ConsumeXML`, `ConsumeText`, `Headers` undefined

- [ ] **Step 3: Write implementation**

```go
// httpclient/response.go
package httpclient

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"os"
)

// ErrEmptyResponseBody is returned by Consume/SaveToFile when Body is nil
// (e.g. after streaming via WithOutput).
var ErrEmptyResponseBody = errors.New("response body is empty")

// Response holds the result of an HTTP request.
type Response struct {
	StatusCode   int
	Body         []byte      // nil when WithOutput was used (body streamed directly)
	Headers      http.Header // response headers — always populated on non-streaming responses
	Error        error
	SuccessCodes []int
	raw          *http.Response // underlying response; body already closed after Execute
}

func (r *Response) isSuccessStatus() bool {
	for _, c := range r.SuccessCodes {
		if r.StatusCode == c {
			return true
		}
	}
	return false
}

func (r *Response) statusError() error {
	body := ""
	if r.Body != nil {
		body = string(r.Body)
	}
	return fmt.Errorf("response status not OK: %d, body: %s", r.StatusCode, body)
}

// IsSuccess returns true when there is no transport error and status is in SuccessCodes.
func (r *Response) IsSuccess() bool {
	return r.Error == nil && r.isSuccessStatus()
}

// IsError returns (true, err) when there is a transport error or status not in SuccessCodes.
func (r *Response) IsError() (bool, error) {
	if r.Error != nil {
		return true, r.Error
	}
	if !r.isSuccessStatus() {
		return true, r.statusError()
	}
	return false, nil
}

// HttpCode returns the HTTP status code.
func (r *Response) HttpCode() int { return r.StatusCode }

// Consume JSON-unmarshals Body into v. Returns an error if transport failed,
// status is not in SuccessCodes, or Body is nil.
func (r *Response) Consume(v any) error {
	if r.Error != nil {
		return r.Error
	}
	if !r.isSuccessStatus() {
		return r.statusError()
	}
	if r.Body == nil {
		return ErrEmptyResponseBody
	}
	if err := json.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w (body: %s)", err, string(r.Body))
	}
	return nil
}

// ConsumeXML XML-unmarshals Body into v. Same preconditions as Consume.
func (r *Response) ConsumeXML(v any) error {
	if r.Error != nil {
		return r.Error
	}
	if !r.isSuccessStatus() {
		return r.statusError()
	}
	if r.Body == nil {
		return ErrEmptyResponseBody
	}
	if err := xml.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("failed to unmarshal XML response body: %w", err)
	}
	return nil
}

// ConsumeText returns Body as a string. Same preconditions as Consume.
func (r *Response) ConsumeText() (string, error) {
	if r.Error != nil {
		return "", r.Error
	}
	if !r.isSuccessStatus() {
		return "", r.statusError()
	}
	if r.Body == nil {
		return "", ErrEmptyResponseBody
	}
	return string(r.Body), nil
}

// SaveToFile writes Body to the file at path with permission 0644.
// Returns ErrEmptyResponseBody when Body is nil (e.g. after WithOutput streaming).
func (r *Response) SaveToFile(path string) error {
	if r.Error != nil {
		return r.Error
	}
	if !r.isSuccessStatus() {
		return r.statusError()
	}
	if r.Body == nil {
		return ErrEmptyResponseBody
	}
	return os.WriteFile(path, r.Body, 0644)
}

// Raw returns the underlying *http.Response. The Body is already read and closed
// by Execute — do not re-read it. Useful for Trailer, TLS state, etc.
func (r *Response) Raw() *http.Response { return r.raw }
```

- [ ] **Step 4: Run tests — expect all pass**

```
go test ./httpclient/... -run "TestResponse" -v
```
Expected: 9 tests PASS

- [ ] **Step 5: Commit**

```
git add httpclient/response.go httpclient/response_test.go
git commit -m "feat(httpclient): add Response with Headers, ConsumeXML, ConsumeText, Raw"
```

---

## Task 3: Client

**Files:**
- Create: `httpclient/client.go`
- Create: `httpclient/client_test.go`

- [ ] **Step 1: Write failing tests**

```go
// httpclient/client_test.go
package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_defaults(t *testing.T) {
	c := New()
	assert.NotNil(t, c)
	assert.Empty(t, c.baseURL)
	assert.Equal(t, time.Duration(0), c.defaultTimeout)
	assert.NotNil(t, c.defaultHeaders)
}

func TestNew_withBaseURL(t *testing.T) {
	c := New(WithBaseURL("https://api.example.com"))
	assert.Equal(t, "https://api.example.com", c.baseURL)
}

func TestNew_withDefaultTimeout(t *testing.T) {
	c := New(WithDefaultTimeout(5 * time.Second))
	assert.Equal(t, 5*time.Second, c.defaultTimeout)
}

func TestNew_withDefaultHeaders(t *testing.T) {
	c := New(WithDefaultHeaders(map[string]string{"X-App": "test"}))
	assert.Equal(t, "test", c.defaultHeaders.Get("X-App"))
}

func TestNew_withMiddleware_appendsToSlice(t *testing.T) {
	m := NewMiddleware("test", func(next Doer) Doer { return next })
	c := New(WithMiddleware(m))
	assert.Len(t, c.middlewares, 1)
}

func TestNew_withDefaultBearer_storesFn(t *testing.T) {
	fn := func() string { return "tok" }
	c := New(WithDefaultBearer(fn))
	assert.NotNil(t, c.defaultBearer)
	assert.Equal(t, "tok", c.defaultBearer())
}

func TestClient_builderMethods_returnRequestBuilderWithCorrectClient(t *testing.T) {
	c := New(WithBaseURL("https://api.example.com"))
	methods := []*RequestBuilder{
		c.Post("/a"), c.Get("/b"), c.Put("/c"),
		c.Delete("/d"), c.Patch("/e"), c.Options("/f"),
	}
	for _, rb := range methods {
		require.NotNil(t, rb)
		assert.Same(t, c, rb.client)
	}
}

func TestClient_buildHTTPClient_setsTimeout(t *testing.T) {
	c := New()
	hc := c.buildHTTPClient(false, 3*time.Second)
	assert.Equal(t, 3*time.Second, hc.Timeout)
}

func TestClient_buildHTTPClient_zeroTimeout_noTimeout(t *testing.T) {
	c := New()
	hc := c.buildHTTPClient(false, 0)
	assert.Equal(t, time.Duration(0), hc.Timeout)
}

func TestClient_buildHTTPClient_withCookieJar(t *testing.T) {
	c := New(WithCookieJar(http.DefaultClient.Jar))
	hc := c.buildHTTPClient(false, 0)
	assert.Equal(t, http.DefaultClient.Jar, hc.Jar)
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```
go test ./httpclient/... -run "TestNew|TestClient_" -v
```
Expected: `undefined: Client` compile error

- [ ] **Step 3: Write implementation**

```go
// httpclient/client.go
package httpclient

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"
)

// Client holds shared configuration and the middleware stack for a group of requests.
// Create one per upstream service and reuse it across requests — it is safe for concurrent use
// after construction.
type Client struct {
	baseURL          string
	defaultHeaders   http.Header
	defaultTimeout   time.Duration
	defaultBearer    func() string // called per request to get the current token; nil if not set
	defaultBasicUser string
	defaultBasicPass string
	middlewares      []Middleware
	cookieJar        http.CookieJar
	transport        http.RoundTripper // nil = http.DefaultTransport
}

// ClientOption configures a Client during New().
type ClientOption func(*Client)

// New creates a Client with the given options.
func New(opts ...ClientOption) *Client {
	c := &Client{defaultHeaders: http.Header{}}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithBaseURL sets a base URL prepended to all relative request paths.
func WithBaseURL(base string) ClientOption {
	return func(c *Client) { c.baseURL = base }
}

// WithDefaultTimeout sets the timeout used for requests that don't set their own.
func WithDefaultTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.defaultTimeout = d }
}

// WithDefaultHeaders merges headers sent with every request from this client.
func WithDefaultHeaders(headers map[string]string) ClientOption {
	return func(c *Client) {
		for k, v := range headers {
			c.defaultHeaders.Set(k, v)
		}
	}
}

// WithDefaultBearer sets a token provider called per request.
// Ignored when the request already has an Authorization header.
func WithDefaultBearer(fn func() string) ClientOption {
	return func(c *Client) { c.defaultBearer = fn }
}

// WithDefaultBasicAuth sets Basic auth credentials sent with every request.
// Ignored when the request already has an Authorization header.
func WithDefaultBasicAuth(user, pass string) ClientOption {
	return func(c *Client) {
		c.defaultBasicUser = user
		c.defaultBasicPass = pass
	}
}

// WithMiddleware appends a middleware to the transport stack.
// Middlewares are applied in registration order: first registered = outermost.
func WithMiddleware(m Middleware) ClientOption {
	return func(c *Client) { c.middlewares = append(c.middlewares, m) }
}

// WithCookieJar attaches a cookie jar — cookies are sent and stored automatically.
func WithCookieJar(jar http.CookieJar) ClientOption {
	return func(c *Client) { c.cookieJar = jar }
}

// WithSkipTLS disables TLS certificate verification for all requests from this client.
func WithSkipTLS() ClientOption {
	return func(c *Client) {
		c.transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
}

// WithTLSConfig sets a custom TLS configuration.
func WithTLSConfig(cfg *tls.Config) ClientOption {
	return func(c *Client) { c.transport = &http.Transport{TLSClientConfig: cfg} }
}

// WithProxy sets an HTTP proxy URL for all requests from this client.
func WithProxy(proxyURL string) ClientOption {
	return func(c *Client) {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return
		}
		c.transport = &http.Transport{Proxy: http.ProxyURL(parsed)}
	}
}

// WithTransport replaces the underlying http.RoundTripper (useful for testing).
func WithTransport(t http.RoundTripper) ClientOption {
	return func(c *Client) { c.transport = t }
}

// buildHTTPClient constructs a net/http.Client for a single request execution.
// The transport (connection pool) is shared unless skipTLS overrides it.
func (c *Client) buildHTTPClient(skipTLS bool, timeout time.Duration) *http.Client {
	transport := c.transport
	if skipTLS && transport == nil {
		transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	hc := &http.Client{Jar: c.cookieJar, Transport: transport}
	if timeout > 0 {
		hc.Timeout = timeout
	}
	return hc
}

// Do creates a RequestBuilder for any HTTP method and path.
func (c *Client) Do(method Method, path string) *RequestBuilder {
	return newRequestBuilder(c, method, path)
}

func (c *Client) Post(path string) *RequestBuilder    { return c.Do(MethodPost, path) }
func (c *Client) Get(path string) *RequestBuilder     { return c.Do(MethodGet, path) }
func (c *Client) Put(path string) *RequestBuilder     { return c.Do(MethodPut, path) }
func (c *Client) Delete(path string) *RequestBuilder  { return c.Do(MethodDelete, path) }
func (c *Client) Patch(path string) *RequestBuilder   { return c.Do(MethodPatch, path) }
func (c *Client) Options(path string) *RequestBuilder { return c.Do(MethodOptions, path) }
```

- [ ] **Step 4: Run tests — expect all pass**

```
go test ./httpclient/... -run "TestNew|TestClient_" -v
```
Expected: all pass (note: `newRequestBuilder` is defined in Task 4)

- [ ] **Step 5: Commit**

```
git add httpclient/client.go httpclient/client_test.go
git commit -m "feat(httpclient): add Client struct with New() and all ClientOption funcs"
```

---

## Task 4: Request Builder

**Files:**
- Create: `httpclient/request.go` (replaces `http.go`)
- Create: `httpclient/request_test.go`

- [ ] **Step 1: Write failing tests**

```go
// httpclient/request_test.go
package httpclient

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testClient() *Client { return New(WithBaseURL("https://api.example.com")) }

func TestRequestBuilder_WithBody_setsJSONMode(t *testing.T) {
	rb := testClient().Post("/users").WithBody(map[string]string{"name": "alice"})
	assert.Equal(t, bodyModeJSON, rb.bodyMode)
	assert.NotNil(t, rb.body)
}

func TestRequestBuilder_WithForm_setsFormMode(t *testing.T) {
	rb := testClient().Post("/login").WithForm(map[string]string{"user": "bob"})
	assert.Equal(t, bodyModeForm, rb.bodyMode)
}

func TestRequestBuilder_WithRawBody_setsRawModeAndContentType(t *testing.T) {
	rb := testClient().Post("/xml").WithRawBody([]byte("<x/>"), "application/xml")
	assert.Equal(t, bodyModeRaw, rb.bodyMode)
	assert.Equal(t, "application/xml", rb.rawContentType)
	assert.Equal(t, []byte("<x/>"), rb.rawBody)
}

func TestRequestBuilder_WithGraphQL_setsJSONBodyWithQueryKey(t *testing.T) {
	rb := testClient().Post("/gql").WithGraphQL("{ users { id } }", nil)
	assert.Equal(t, bodyModeJSON, rb.bodyMode)
	body := rb.body.(map[string]any)
	assert.Equal(t, "{ users { id } }", body["query"])
}

func TestRequestBuilder_WithMultipart_setsMultipartMode(t *testing.T) {
	rb := testClient().Post("/upload").WithMultipart(Field("name", "avatar"))
	assert.Equal(t, bodyModeMultipart, rb.bodyMode)
	assert.Len(t, rb.multipartParts, 1)
}

func TestRequestBuilder_WithTimeout_overridesDefault(t *testing.T) {
	rb := testClient().Get("/users").WithTimeout(3 * time.Second)
	assert.Equal(t, 3*time.Second, rb.timeout)
}

func TestRequestBuilder_WithHeaders_mergesHeaders(t *testing.T) {
	rb := testClient().Get("/users").WithHeaders(map[string]string{"X-Foo": "bar"})
	assert.Equal(t, "bar", rb.headers.Get("X-Foo"))
}

func TestRequestBuilder_WithBearer_setsAuthHeader(t *testing.T) {
	rb := testClient().Get("/me").WithBearer("mytoken")
	assert.Equal(t, "Bearer mytoken", rb.headers.Get("Authorization"))
}

func TestRequestBuilder_WithBasicAuth_setsBase64AuthHeader(t *testing.T) {
	rb := testClient().Get("/me").WithBasicAuth("user", "pass")
	assert.True(t, strings.HasPrefix(rb.headers.Get("Authorization"), "Basic "))
}

func TestRequestBuilder_WithoutMiddleware_addsToSkipMap(t *testing.T) {
	rb := testClient().Get("/health").WithoutMiddleware(CircuitBreakerKey, RetryKey)
	assert.True(t, rb.skipMiddleware[CircuitBreakerKey])
	assert.True(t, rb.skipMiddleware[RetryKey])
}

func TestRequestBuilder_WithSuccessCodes_replacesDefault(t *testing.T) {
	rb := testClient().Get("/ok").WithSuccessCodes([]int{200, 201})
	assert.Equal(t, []int{200, 201}, rb.successCodes)
}

func TestRequestBuilder_WithQueryParams_setsMap(t *testing.T) {
	rb := testClient().Get("/search").WithQueryParams(map[string]string{"q": "go"})
	assert.Equal(t, "go", rb.queryParams["q"])
}

func TestRequestBuilder_WithQueryParam_alias_setsMap(t *testing.T) {
	rb := testClient().Get("/search").WithQueryParam(map[string]string{"q": "go"})
	assert.Equal(t, "go", rb.queryParams["q"])
}

func TestField_constructor(t *testing.T) {
	f := Field("key", "val")
	assert.Equal(t, "key", f.field)
	assert.Equal(t, "val", f.value)
	assert.False(t, f.isFile)
}

func TestFileFromReader_constructor(t *testing.T) {
	r := strings.NewReader("content")
	f := FileFromReader("file", "photo.jpg", r)
	assert.Equal(t, "file", f.field)
	assert.Equal(t, "photo.jpg", f.filename)
	assert.True(t, f.isFile)
	assert.NotNil(t, f.reader)
}

func TestFileFromPath_constructor(t *testing.T) {
	f := FileFromPath("resume", "/tmp/cv.pdf")
	assert.Equal(t, "resume", f.field)
	assert.Equal(t, "/tmp/cv.pdf", f.filename)
	assert.True(t, f.isFile)
	assert.Nil(t, f.reader) // opened lazily in buildBody
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```
go test ./httpclient/... -run "TestRequestBuilder|TestField|TestFileFrom" -v
```
Expected: `undefined: bodyModeJSON` compile error

- [ ] **Step 3: Write implementation**

```go
// httpclient/request.go
package httpclient

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"github.com/ewinjuman/go-lib/v2/logger"
)

// Method represents an HTTP verb.
type Method string

const (
	MethodPost    Method = "POST"
	MethodGet     Method = "GET"
	MethodPut     Method = "PUT"
	MethodDelete  Method = "DELETE"
	MethodPatch   Method = "PATCH"
	MethodOptions Method = "OPTIONS"
)

func (m Method) String() string { return string(m) }

type bodyMode int

const (
	bodyModeNone      bodyMode = iota
	bodyModeJSON
	bodyModeForm
	bodyModeMultipart
	bodyModeRaw
)

// MultipartPart is one part of a multipart/form-data request body.
type MultipartPart struct {
	field    string
	filename string
	reader   io.Reader // non-nil for FileFromReader; nil for FileFromPath (opened lazily)
	value    string    // text value for fields; file path for FileFromPath
	isFile   bool
}

// Field creates a text form field for a multipart request.
func Field(key, value string) MultipartPart {
	return MultipartPart{field: key, value: value}
}

// FileFromReader creates a file part using an existing io.Reader.
func FileFromReader(field, filename string, r io.Reader) MultipartPart {
	return MultipartPart{field: field, filename: filename, reader: r, isFile: true}
}

// FileFromPath creates a file part whose content is read from disk during Execute.
func FileFromPath(field, path string) MultipartPart {
	return MultipartPart{field: field, filename: path, value: path, isFile: true}
}

// RequestBuilder holds per-request configuration. Not safe for concurrent use.
// Created by Client.Post/Get/etc., not directly.
type RequestBuilder struct {
	client         *Client
	method         Method
	path           string
	body           any
	bodyMode       bodyMode
	rawBody        []byte
	rawContentType string
	multipartParts []MultipartPart
	pathParams     map[string]string
	queryParams    map[string]string
	headers        http.Header
	cookies        []*http.Cookie
	ctx            context.Context
	timeout        time.Duration
	output         io.Writer
	successCodes   []int
	requestID      string
	debug          bool
	skipTLS        bool
	skipMiddleware map[string]bool
	writer         logger.Writer
}

func newRequestBuilder(c *Client, method Method, path string) *RequestBuilder {
	return &RequestBuilder{
		client:       c,
		method:       method,
		path:         path,
		headers:      c.defaultHeaders.Clone(),
		successCodes: []int{200},
	}
}

func (rb *RequestBuilder) WithBody(v any) *RequestBuilder {
	rb.body = v
	rb.bodyMode = bodyModeJSON
	return rb
}

func (rb *RequestBuilder) WithForm(v any) *RequestBuilder {
	rb.body = v
	rb.bodyMode = bodyModeForm
	return rb
}

func (rb *RequestBuilder) WithMultipart(parts ...MultipartPart) *RequestBuilder {
	rb.multipartParts = parts
	rb.bodyMode = bodyModeMultipart
	return rb
}

func (rb *RequestBuilder) WithRawBody(b []byte, contentType string) *RequestBuilder {
	rb.rawBody = b
	rb.rawContentType = contentType
	rb.bodyMode = bodyModeRaw
	return rb
}

// WithGraphQL sets a JSON body {"query": q, "variables": vars}.
func (rb *RequestBuilder) WithGraphQL(q string, vars map[string]any) *RequestBuilder {
	rb.body = map[string]any{"query": q, "variables": vars}
	rb.bodyMode = bodyModeJSON
	return rb
}

// WithQueryParams sets the query string parameters for this request.
func (rb *RequestBuilder) WithQueryParams(params map[string]string) *RequestBuilder {
	rb.queryParams = params
	return rb
}

// WithQueryParam is an alias for WithQueryParams kept for backward compatibility.
func (rb *RequestBuilder) WithQueryParam(params map[string]string) *RequestBuilder {
	return rb.WithQueryParams(params)
}

func (rb *RequestBuilder) WithPathParam(key, val string) *RequestBuilder {
	if rb.pathParams == nil {
		rb.pathParams = make(map[string]string)
	}
	rb.pathParams[key] = val
	return rb
}

func (rb *RequestBuilder) WithHeaders(headers map[string]string) *RequestBuilder {
	for k, v := range headers {
		rb.headers.Set(k, v)
	}
	return rb
}

func (rb *RequestBuilder) WithBearer(token string) *RequestBuilder {
	rb.headers.Set("Authorization", "Bearer "+token)
	return rb
}

func (rb *RequestBuilder) WithBasicAuth(user, pass string) *RequestBuilder {
	token := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	rb.headers.Set("Authorization", "Basic "+token)
	return rb
}

func (rb *RequestBuilder) WithCookie(name, val string) *RequestBuilder {
	rb.cookies = append(rb.cookies, &http.Cookie{Name: name, Value: val})
	return rb
}

func (rb *RequestBuilder) WithTimeout(d time.Duration) *RequestBuilder {
	rb.timeout = d
	return rb
}

// WithOutput streams the response body directly to w instead of buffering in Response.Body.
// After Execute(), Response.Body is nil — do not call Consume() or SaveToFile() on the result.
func (rb *RequestBuilder) WithOutput(w io.Writer) *RequestBuilder {
	rb.output = w
	return rb
}

// WithoutMiddleware skips the named middlewares for this request only.
// Pass CircuitBreakerKey, RetryKey, LoggingKey, or any custom key.
func (rb *RequestBuilder) WithoutMiddleware(keys ...string) *RequestBuilder {
	if rb.skipMiddleware == nil {
		rb.skipMiddleware = make(map[string]bool)
	}
	for _, k := range keys {
		rb.skipMiddleware[k] = true
	}
	return rb
}

func (rb *RequestBuilder) WithSuccessCodes(codes []int) *RequestBuilder {
	if len(codes) > 0 {
		rb.successCodes = codes
	}
	return rb
}

func (rb *RequestBuilder) WithRequestID(id string) *RequestBuilder {
	rb.requestID = id
	return rb
}

func (rb *RequestBuilder) WithDebug(debug bool) *RequestBuilder {
	rb.debug = debug
	return rb
}

func (rb *RequestBuilder) WithSkipTLS() *RequestBuilder {
	rb.skipTLS = true
	return rb
}

func (rb *RequestBuilder) WithContext(ctx context.Context) *RequestBuilder {
	rb.ctx = ctx
	return rb
}

func (rb *RequestBuilder) WithWriter(w logger.Writer) *RequestBuilder {
	rb.writer = w
	return rb
}
```

- [ ] **Step 4: Run tests — expect all pass**

```
go test ./httpclient/... -run "TestRequestBuilder|TestField|TestFileFrom" -v
```
Expected: all pass

- [ ] **Step 5: Commit**

```
git add httpclient/request.go httpclient/request_test.go
git commit -m "feat(httpclient): add RequestBuilder with all With* methods and MultipartPart helpers"
```

---

## Task 5: Body Building & URL Preparation

**Files:**
- Create: `httpclient/execute.go` (initial — helpers only, Execute placeholder)
- Create: `httpclient/execute_test.go` (replaces old one — body/URL tests only for now)

- [ ] **Step 1: Write failing tests**

```go
// httpclient/execute_test.go
package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newJSONServer creates an httptest.Server that always replies with status + JSON body.
func newJSONServer(t *testing.T, status int, body any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

// ─── buildBody ───────────────────────────────────────────────────────────────

func TestBuildBody_none_returnsNilReader(t *testing.T) {
	rb := New().Get("/")
	reader, ct, err := rb.buildBody()
	require.NoError(t, err)
	assert.Nil(t, reader)
	assert.Empty(t, ct)
}

func TestBuildBody_json_encodesBodyAsJSON(t *testing.T) {
	rb := New().Post("/").WithBody(map[string]string{"k": "v"})
	reader, ct, err := rb.buildBody()
	require.NoError(t, err)
	assert.Empty(t, ct) // Content-Type set via header separately
	b, _ := io.ReadAll(reader)
	var got map[string]string
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, "v", got["k"])
}

func TestBuildBody_form_encodesURLEncoded(t *testing.T) {
	rb := New().Post("/").WithForm(map[string]string{"user": "alice"})
	reader, ct, err := rb.buildBody()
	require.NoError(t, err)
	assert.Equal(t, "application/x-www-form-urlencoded", ct)
	b, _ := io.ReadAll(reader)
	assert.Contains(t, string(b), "user=alice")
}

func TestBuildBody_raw_returnsContentType(t *testing.T) {
	rb := New().Post("/").WithRawBody([]byte("<xml/>"), "application/xml")
	reader, ct, err := rb.buildBody()
	require.NoError(t, err)
	assert.Equal(t, "application/xml", ct)
	b, _ := io.ReadAll(reader)
	assert.Equal(t, "<xml/>", string(b))
}

func TestBuildBody_multipart_containsFieldAndFile(t *testing.T) {
	rb := New().Post("/").WithMultipart(
		Field("name", "avatar"),
		FileFromReader("file", "photo.jpg", strings.NewReader("IMGDATA")),
	)
	reader, ct, err := rb.buildBody()
	require.NoError(t, err)
	assert.Contains(t, ct, "multipart/form-data")
	b, _ := io.ReadAll(reader)
	s := string(b)
	assert.Contains(t, s, "avatar")
	assert.Contains(t, s, "IMGDATA")
}

// ─── buildURL ────────────────────────────────────────────────────────────────

func TestBuildURL_absoluteURL_noBaseURL(t *testing.T) {
	rb := New().Get("https://api.example.com/users")
	assert.Equal(t, "https://api.example.com/users", rb.buildURL())
}

func TestBuildURL_relativeURL_prependsBaseURL(t *testing.T) {
	rb := New(WithBaseURL("https://api.example.com")).Get("/users")
	assert.Equal(t, "https://api.example.com/users", rb.buildURL())
}

func TestBuildURL_pathParam_replaced(t *testing.T) {
	rb := New(WithBaseURL("https://api.example.com")).Get("/users/:id").WithPathParam("id", "42")
	assert.Equal(t, "https://api.example.com/users/42", rb.buildURL())
}

func TestBuildURL_multiplePathParams(t *testing.T) {
	rb := New().Get("https://api.example.com/orgs/:org/users/:id").
		WithPathParam("org", "acme").
		WithPathParam("id", "99")
	assert.Equal(t, "https://api.example.com/orgs/acme/users/99", rb.buildURL())
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```
go test ./httpclient/... -run "TestBuildBody|TestBuildURL" -v
```
Expected: `undefined: buildBody` or `undefined: buildURL`

- [ ] **Step 3: Create execute.go with helpers and Execute placeholder**

```go
// httpclient/execute.go
package httpclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	Error "github.com/ewinjuman/go-lib/v2/apperror"
	"github.com/ewinjuman/go-lib/v2/logger"
	"github.com/ewinjuman/go-lib/v2/utils/convert"
	"github.com/google/uuid"
)

var (
	jsonCheck = regexp.MustCompile(`(?i:(application|text)/(.*json.*)(;|$))`)
	xmlCheck  = regexp.MustCompile(`(?i:(application|text)/(.*xml.*)(;|$))`)
	bufPool   = sync.Pool{New: func() any { return new(bytes.Buffer) }}
)

// buildURL joins the client base URL with rb.path and replaces :param placeholders.
func (rb *RequestBuilder) buildURL() string {
	p := rb.path
	for key, val := range rb.pathParams {
		p = strings.ReplaceAll(p, ":"+key, val)
	}
	if rb.client.baseURL != "" &&
		!strings.HasPrefix(p, "http://") &&
		!strings.HasPrefix(p, "https://") {
		return strings.TrimRight(rb.client.baseURL, "/") + "/" + strings.TrimLeft(p, "/")
	}
	return p
}

// buildBody encodes rb.body into an io.Reader.
// Returns the reader, an optional Content-Type override, and any error.
func (rb *RequestBuilder) buildBody() (io.Reader, string, error) {
	switch rb.bodyMode {
	case bodyModeNone:
		return nil, "", nil

	case bodyModeJSON:
		if rb.body == nil {
			return nil, "", nil
		}
		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		if err := json.NewEncoder(buf).Encode(rb.body); err != nil {
			bufPool.Put(buf)
			return nil, "", err
		}
		data := make([]byte, buf.Len())
		copy(data, buf.Bytes())
		bufPool.Put(buf)
		return bytes.NewReader(data), "", nil

	case bodyModeForm:
		var m map[string]string
		convert.ObjectToObject(rb.body, &m)
		form := url.Values{}
		for k, v := range m {
			form.Set(k, v)
		}
		return strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", nil

	case bodyModeMultipart:
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for _, part := range rb.multipartParts {
			if part.isFile {
				var r io.Reader
				if part.reader != nil {
					r = part.reader
				} else {
					f, err := os.Open(part.value)
					if err != nil {
						return nil, "", fmt.Errorf("open multipart file %q: %w", part.value, err)
					}
					defer f.Close()
					r = f
				}
				fw, err := w.CreateFormFile(part.field, part.filename)
				if err != nil {
					return nil, "", err
				}
				if _, err = io.Copy(fw, r); err != nil {
					return nil, "", err
				}
			} else {
				if err := w.WriteField(part.field, part.value); err != nil {
					return nil, "", err
				}
			}
		}
		w.Close()
		return &buf, w.FormDataContentType(), nil

	case bodyModeRaw:
		return bytes.NewReader(rb.rawBody), rb.rawContentType, nil
	}
	return nil, "", nil
}

// applyDefaults fills in headers and auth that were not set per-request.
func (rb *RequestBuilder) applyDefaults() {
	if rb.requestID != "" {
		rb.headers.Set("X-Request-ID", rb.requestID)
	} else if rb.headers.Get("X-Request-ID") == "" {
		rb.headers.Set("X-Request-ID", uuid.New().String())
	}
	if rb.headers.Get("Content-Type") == "" && rb.method != MethodGet && rb.bodyMode == bodyModeJSON {
		rb.headers.Set("Content-Type", "application/json")
	}
	if rb.headers.Get("Authorization") == "" {
		if rb.client.defaultBearer != nil {
			rb.headers.Set("Authorization", "Bearer "+rb.client.defaultBearer())
		} else if rb.client.defaultBasicUser != "" {
			token := base64.StdEncoding.EncodeToString(
				[]byte(rb.client.defaultBasicUser + ":" + rb.client.defaultBasicPass),
			)
			rb.headers.Set("Authorization", "Basic "+token)
		}
	}
	if rb.debug && rb.writer == nil {
		rb.writer = &logger.DefaultWriter{ID: rb.requestID}
	}
}

func (rb *RequestBuilder) logResponse(ctx context.Context, response *Response, header http.Header, rawURL string, responseTime time.Duration) {
	contentType := header.Get("Content-Type")
	var result any
	switch {
	case xmlCheck.MatchString(contentType):
		if err := xml.Unmarshal(response.Body, &result); err != nil {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, response.StatusCode, string(response.Body), header, responseTime, nil)
		} else {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, response.StatusCode, result, header, responseTime, nil)
		}
	default:
		if err := json.Unmarshal(response.Body, &result); err != nil {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, response.StatusCode, string(response.Body), header, responseTime, nil)
		} else {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, response.StatusCode, result, header, responseTime, nil)
		}
	}
}

// Execute — implemented in Task 6.
func (rb *RequestBuilder) Execute() *Response {
	panic("Execute not yet implemented")
}

// keep unused imports happy until Task 6
var _ = Error.IsTimeout
var _ = time.Now
```

- [ ] **Step 4: Run body/URL tests — expect all pass**

```
go test ./httpclient/... -run "TestBuildBody|TestBuildURL" -v
```
Expected: all pass

- [ ] **Step 5: Build check**

```
go build ./httpclient/...
```
Expected: compiles (Execute panics at runtime, not compile time)

- [ ] **Step 6: Commit**

```
git add httpclient/execute.go httpclient/execute_test.go
git commit -m "feat(httpclient): add buildBody, buildURL, applyDefaults"
```

---

## Task 6: Execute() Flow

**Files:**
- Modify: `httpclient/execute.go` — replace Execute() placeholder
- Extend: `httpclient/execute_test.go` — append integration tests

- [ ] **Step 1: Append integration tests to execute_test.go**

```go
// ─── Execute() integration ───────────────────────────────────────────────────

func TestExecute_GET_success(t *testing.T) {
	srv := newJSONServer(t, 200, map[string]string{"status": "ok"})
	defer srv.Close()

	resp := New().Get(srv.URL + "/ping").Execute()

	require.NoError(t, resp.Error)
	assert.Equal(t, 200, resp.StatusCode)
	assert.NotEmpty(t, resp.Body)
	assert.True(t, resp.IsSuccess())
}

func TestExecute_POST_sendsJSONBodyAndContentTypeHeader(t *testing.T) {
	type payload struct{ Name string }
	var received payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	New().Post(srv.URL).WithBody(payload{"alice"}).Execute()
	assert.Equal(t, "alice", received.Name)
}

func TestExecute_responseHeaders_populated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "hello")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	resp := New().Get(srv.URL).Execute()
	assert.Equal(t, "hello", resp.Headers.Get("X-Custom"))
}

func TestExecute_withOutput_streamsBodyAndBodyIsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("stream data"))
	}))
	defer srv.Close()

	var buf strings.Builder
	resp := New().Get(srv.URL).WithOutput(&buf).Execute()
	require.NoError(t, resp.Error)
	assert.Nil(t, resp.Body)
	assert.Equal(t, "stream data", buf.String())
}

func TestExecute_nonSuccessStatus_isError(t *testing.T) {
	srv := newJSONServer(t, 404, map[string]string{"error": "not found"})
	defer srv.Close()

	resp := New().Get(srv.URL).Execute()
	assert.Equal(t, 404, resp.StatusCode)
	isErr, err := resp.IsError()
	assert.True(t, isErr)
	assert.Error(t, err)
}

func TestExecute_withBaseURL_relativePathJoined(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
	}))
	defer srv.Close()

	New(WithBaseURL(srv.URL)).Get("/users/42").Execute()
	assert.Equal(t, "/users/42", gotPath)
}

func TestExecute_withQueryParams_sentInURL(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("search")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	New().Get(srv.URL).WithQueryParams(map[string]string{"search": "golang"}).Execute()
	assert.Equal(t, "golang", gotQuery)
}

func TestExecute_withContext_cancelledReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	resp := New().Get(srv.URL).WithContext(ctx).Execute()
	assert.Error(t, resp.Error)
}

func TestExecute_withCookie_sentToServer(t *testing.T) {
	var gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie("session")
		if c != nil {
			gotCookie = c.Value
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	New().Get(srv.URL).WithCookie("session", "abc123").Execute()
	assert.Equal(t, "abc123", gotCookie)
}

func TestExecute_withDefaultBearer_setsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	New(WithDefaultBearer(func() string { return "mytoken" })).Get(srv.URL).Execute()
	assert.Equal(t, "Bearer mytoken", gotAuth)
}
```

- [ ] **Step 2: Run tests — expect panic from placeholder**

```
go test ./httpclient/... -run "TestExecute" -v
```
Expected: panic `Execute not yet implemented`

- [ ] **Step 3: Replace the Execute() placeholder in execute.go**

Replace:
```go
// Execute — implemented in Task 6.
func (rb *RequestBuilder) Execute() *Response {
	panic("Execute not yet implemented")
}

// keep unused imports happy until Task 6
var _ = Error.IsTimeout
var _ = time.Now
```

With:
```go
// Execute sends the HTTP request through the composed middleware stack
// and returns a Response. It is the terminal step of the fluent builder.
func (rb *RequestBuilder) Execute() *Response {
	rb.applyDefaults()
	response := &Response{SuccessCodes: rb.successCodes}

	bodyReader, overrideContentType, err := rb.buildBody()
	if err != nil {
		response.Error = err
		return response
	}

	rawURL := rb.buildURL()
	ctx := rb.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, rb.method.String(), rawURL, bodyReader)
	if err != nil {
		response.Error = err
		return response
	}

	for k, vals := range rb.headers {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}
	if overrideContentType != "" {
		req.Header.Set("Content-Type", overrideContentType)
	}

	if len(rb.queryParams) > 0 {
		q := req.URL.Query()
		for k, v := range rb.queryParams {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	for _, c := range rb.cookies {
		req.AddCookie(c)
	}

	timeout := rb.timeout
	if timeout == 0 {
		timeout = rb.client.defaultTimeout
	}

	hc := rb.client.buildHTTPClient(rb.skipTLS, timeout)
	doer := Apply(hc, rb.client.middlewares, rb.skipMiddleware)

	if rb.debug && rb.writer != nil {
		rb.writer.Print(ctx, "http_request", rb.method.String(), rawURL, rb.body, rb.headers, rb.queryParams)
	}

	start := time.Now()
	resp, errDo := doer.Do(req)
	responseTime := time.Since(start)

	if errDo != nil {
		outErr := errDo
		if Error.IsTimeout(errDo) {
			outErr = Error.ErrDeadlineExceeded
		}
		response.Error = outErr
		if rb.debug && rb.writer != nil {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, 0, nil, http.Header{}, responseTime, outErr)
		}
		return response
	}
	defer resp.Body.Close()

	response.StatusCode = resp.StatusCode
	response.Headers = resp.Header
	response.raw = resp

	if rb.output != nil {
		_, copyErr := io.Copy(rb.output, resp.Body)
		if rb.debug && rb.writer != nil {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, response.StatusCode, "[streamed]", resp.Header, responseTime, copyErr)
		}
		if copyErr != nil {
			response.Error = copyErr
		}
		return response
	}

	bodyBytes, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		response.Error = errRead
		return response
	}
	response.Body = bodyBytes

	if rb.debug && rb.writer != nil {
		rb.logResponse(ctx, response, resp.Header, rawURL, responseTime)
	}

	return response
}
```

- [ ] **Step 4: Run all execute tests — expect all pass**

```
go test ./httpclient/... -run "TestExecute|TestBuildBody|TestBuildURL" -v
```
Expected: all pass

- [ ] **Step 5: Commit**

```
git add httpclient/execute.go httpclient/execute_test.go
git commit -m "feat(httpclient): implement Execute() with middleware stack, streaming, context, cookies"
```

---

## Task 7: Package-Level Shortcuts

**Files:**
- Create: `httpclient/http.go`
- Delete: `httpclient/http_client.go`

- [ ] **Step 1: Delete old http_client.go**

```
git rm httpclient/http_client.go
```

- [ ] **Step 2: Create http.go**

```go
// httpclient/http.go
package httpclient

import "time"

// defaultClient is the package-level client used by Post/Get/etc. shortcuts.
// It has no base URL and no middleware. For production, create a named client
// with New() and configure it explicitly.
var defaultClient = New(WithDefaultTimeout(30 * time.Second))

// Do creates a RequestBuilder on the default client using any HTTP method.
func Do(method Method, url string) *RequestBuilder { return defaultClient.Do(method, url) }

func Post(url string) *RequestBuilder    { return defaultClient.Post(url) }
func Get(url string) *RequestBuilder     { return defaultClient.Get(url) }
func Put(url string) *RequestBuilder     { return defaultClient.Put(url) }
func Delete(url string) *RequestBuilder  { return defaultClient.Delete(url) }
func Patch(url string) *RequestBuilder   { return defaultClient.Patch(url) }
func Options(url string) *RequestBuilder { return defaultClient.Options(url) }
```

- [ ] **Step 3: Run full httpclient tests**

```
go test ./httpclient/... -v
```
Expected: all pass, no compile errors

- [ ] **Step 4: Commit**

```
git add httpclient/http.go
git commit -m "feat(httpclient): add package-level shortcuts; delete http_client.go"
```

---

## Task 8: Retry Middleware

**Files:**
- Create: `httpclient/retry.go`
- Create: `httpclient/retry_test.go`

- [ ] **Step 1: Write failing tests**

```go
// httpclient/retry_test.go
package httpclient

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixedBackoff_alwaysReturnsSameDelay(t *testing.T) {
	bf := FixedBackoff(100 * time.Millisecond)
	assert.Equal(t, 100*time.Millisecond, bf(0))
	assert.Equal(t, 100*time.Millisecond, bf(5))
}

func TestExponentialBackoff_doublesEachAttempt(t *testing.T) {
	bf := ExponentialBackoff(100*time.Millisecond, 2.0)
	assert.Equal(t, 100*time.Millisecond, bf(0))
	assert.Equal(t, 200*time.Millisecond, bf(1))
	assert.Equal(t, 400*time.Millisecond, bf(2))
}

func TestExponentialWithJitter_withinExpectedRange(t *testing.T) {
	bf := ExponentialWithJitter(100*time.Millisecond, 2.0)
	for i := 0; i < 20; i++ {
		// attempt=1 → base=200ms; jitter adds 0–200ms → total 0–400ms
		d := bf(1)
		assert.GreaterOrEqual(t, int64(d), int64(0))
		assert.LessOrEqual(t, int64(d), int64(400*time.Millisecond))
	}
}

func TestRetryOnError_retriesOnlyOnNetworkError(t *testing.T) {
	assert.True(t, RetryOnError(nil, errors.New("dial tcp")))
	assert.False(t, RetryOnError(&http.Response{StatusCode: 500}, nil))
}

func TestRetryOnStatus_retriesOnMatchingStatus(t *testing.T) {
	fn := RetryOnStatus(429, 503)
	assert.True(t, fn(&http.Response{StatusCode: 429}, nil))
	assert.True(t, fn(&http.Response{StatusCode: 503}, nil))
	assert.False(t, fn(&http.Response{StatusCode: 200}, nil))
	assert.False(t, fn(nil, errors.New("network")))
}

func TestRetryOnAny_retriesOnErrorOrServerError(t *testing.T) {
	assert.True(t, RetryOnAny(nil, errors.New("err")))
	assert.True(t, RetryOnAny(&http.Response{StatusCode: 503}, nil))
	assert.False(t, RetryOnAny(&http.Response{StatusCode: 200}, nil))
}

func TestRetryMiddleware_retriesUntilSuccess(t *testing.T) {
	var calls atomic.Int32
	base := DoerFunc(func(r *http.Request) (*http.Response, error) {
		n := calls.Add(1)
		if n < 3 {
			return nil, errors.New("temporary failure")
		}
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	})
	m := RetryMiddleware(RetryConfig{
		MaxAttempts: 3,
		Backoff:     FixedBackoff(0),
		RetryOn:     RetryOnError,
	})
	composed := Apply(base, []Middleware{m}, nil)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://example.com", nil)
	resp, err := composed.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, int32(3), calls.Load())
}

func TestRetryMiddleware_doesNotRetryOnSuccess(t *testing.T) {
	var calls atomic.Int32
	base := DoerFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	})
	m := RetryMiddleware(RetryConfig{MaxAttempts: 3, Backoff: FixedBackoff(0), RetryOn: RetryOnError})
	composed := Apply(base, []Middleware{m}, nil)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, _ := composed.Do(req)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, int32(1), calls.Load())
}

func TestRetryMiddleware_stopsOnContextCancellation(t *testing.T) {
	var calls atomic.Int32
	base := DoerFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("fail")
	})
	m := RetryMiddleware(RetryConfig{
		MaxAttempts: 10,
		Backoff:     FixedBackoff(50 * time.Millisecond),
		RetryOn:     RetryOnError,
	})
	composed := Apply(base, []Middleware{m}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)
	_, err := composed.Do(req)
	assert.Error(t, err)
	assert.Less(t, calls.Load(), int32(10))
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```
go test ./httpclient/... -run "TestRetry|TestFixed|TestExponential|TestRetryOn" -v
```
Expected: `undefined: RetryMiddleware`

- [ ] **Step 3: Write implementation**

```go
// httpclient/retry.go
package httpclient

import (
	"math"
	"math/rand"
	"net/http"
	"time"
)

// BackoffFunc returns the delay before the nth retry attempt (0-indexed).
type BackoffFunc func(attempt int) time.Duration

// FixedBackoff waits d before every retry.
func FixedBackoff(d time.Duration) BackoffFunc {
	return func(_ int) time.Duration { return d }
}

// ExponentialBackoff returns base * multiplier^attempt.
func ExponentialBackoff(base time.Duration, multiplier float64) BackoffFunc {
	return func(attempt int) time.Duration {
		return time.Duration(float64(base) * math.Pow(multiplier, float64(attempt)))
	}
}

// ExponentialWithJitter adds uniform random jitter in [0, delay] to ExponentialBackoff.
// This prevents thundering-herd effects when many clients retry simultaneously.
func ExponentialWithJitter(base time.Duration, multiplier float64) BackoffFunc {
	exp := ExponentialBackoff(base, multiplier)
	return func(attempt int) time.Duration {
		d := exp(attempt)
		jitter := time.Duration(rand.Int63n(int64(d) + 1))
		return d + jitter
	}
}

// RetryOnError retries only on transport/network errors (resp is nil).
func RetryOnError(resp *http.Response, err error) bool { return err != nil }

// RetryOnStatus returns a condition that retries when the response status is in codes.
// Does not retry on transport errors.
func RetryOnStatus(codes ...int) func(*http.Response, error) bool {
	set := make(map[int]bool, len(codes))
	for _, c := range codes {
		set[c] = true
	}
	return func(resp *http.Response, err error) bool {
		if err != nil || resp == nil {
			return false
		}
		return set[resp.StatusCode]
	}
}

// RetryOnAny retries on transport errors and on any 5xx response.
func RetryOnAny(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp != nil && resp.StatusCode >= 500
}

// RetryConfig configures the RetryMiddleware.
type RetryConfig struct {
	MaxAttempts int                                        // total attempts including the first; ≤0 means 1
	Backoff     BackoffFunc                                // delay before each retry; nil = no delay
	RetryOn     func(resp *http.Response, err error) bool // nil = never retry
}

// RetryMiddleware retries failed requests according to cfg.
// Context cancellation stops the retry loop immediately.
func RetryMiddleware(cfg RetryConfig) Middleware {
	return NewMiddleware(RetryKey, func(next Doer) Doer {
		return DoerFunc(func(req *http.Request) (*http.Response, error) {
			max := cfg.MaxAttempts
			if max <= 0 {
				max = 1
			}
			var (
				resp *http.Response
				err  error
			)
			for attempt := 0; attempt < max; attempt++ {
				if req.Context() != nil {
					select {
					case <-req.Context().Done():
						return nil, req.Context().Err()
					default:
					}
				}

				resp, err = next.Do(req)

				if attempt == max-1 || cfg.RetryOn == nil || !cfg.RetryOn(resp, err) {
					return resp, err
				}

				// Drain body before retry to allow connection reuse
				if resp != nil && resp.Body != nil {
					resp.Body.Close()
				}

				if cfg.Backoff != nil {
					delay := cfg.Backoff(attempt)
					if delay > 0 {
						timer := time.NewTimer(delay)
						select {
						case <-req.Context().Done():
							timer.Stop()
							return nil, req.Context().Err()
						case <-timer.C:
						}
					}
				}
			}
			return resp, err
		})
	})
}
```

- [ ] **Step 4: Run all retry tests — expect all pass**

```
go test ./httpclient/... -run "TestRetry|TestFixed|TestExponential|TestRetryOn" -v
```
Expected: all pass

- [ ] **Step 5: Commit**

```
git add httpclient/retry.go httpclient/retry_test.go
git commit -m "feat(httpclient): add RetryMiddleware with FixedBackoff, ExponentialBackoff, ExponentialWithJitter"
```

---

## Task 9: Circuit Breaker Middleware

**Files:**
- Extend: `httpclient/circuit_breaker.go` — append `CircuitBreakerMiddleware`
- Create: `httpclient/circuit_breaker_test.go`

- [ ] **Step 1: Write failing tests**

```go
// httpclient/circuit_breaker_test.go
package httpclient

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_closedByDefault(t *testing.T) {
	cb := newCircuitBreaker(nil)
	assert.Equal(t, "CLOSED", cb.State())
	assert.NoError(t, cb.Allow())
}

func TestCircuitBreaker_opensAfterFailureThreshold(t *testing.T) {
	cb := newCircuitBreaker(&CircuitBreakerConfig{FailureThreshold: 2})
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, "OPEN", cb.State())
	assert.Error(t, cb.Allow())
}

func TestCircuitBreaker_transitionsHalfOpenThenClosedOnSuccess(t *testing.T) {
	cb := newCircuitBreaker(&CircuitBreakerConfig{FailureThreshold: 1, RecoveryTimeout: 0})
	cb.RecordFailure()
	require.Equal(t, "OPEN", cb.State())
	// RecoveryTimeout=0 → next Allow() immediately transitions to HALF_OPEN
	require.NoError(t, cb.Allow())
	assert.Equal(t, "HALF_OPEN", cb.State())
	cb.RecordSuccess()
	assert.Equal(t, "CLOSED", cb.State())
}

func TestCircuitBreakerMiddleware_blocksWhenCircuitIsOpen(t *testing.T) {
	cbRegistry.Delete("http://blocked-host")
	called := false
	base := DoerFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	})
	m := CircuitBreakerMiddleware(CircuitBreakerConfig{FailureThreshold: 1})
	composed := Apply(base, []Middleware{m}, nil)

	// Manually open the circuit
	cb := getCircuitBreaker("http://blocked-host/path", nil)
	cb.RecordFailure()

	req, _ := http.NewRequest("GET", "http://blocked-host/path", nil)
	_, err := composed.Do(req)
	assert.Error(t, err)
	assert.False(t, called, "base Doer must not be called when circuit is open")
}

func TestCircuitBreakerMiddleware_records5xxAsFailure(t *testing.T) {
	cbRegistry.Delete("http://fail-host")
	base := DoerFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: http.NoBody}, nil
	})
	m := CircuitBreakerMiddleware(CircuitBreakerConfig{FailureThreshold: 2})
	composed := Apply(base, []Middleware{m}, nil)
	req, _ := http.NewRequest("GET", "http://fail-host/a", nil)
	composed.Do(req)
	composed.Do(req)

	cb := getCircuitBreaker("http://fail-host/a", nil)
	assert.Equal(t, "OPEN", cb.State())
}

func TestCircuitBreakerMiddleware_records4xxAsSuccess(t *testing.T) {
	cbRegistry.Delete("http://client-err-host")
	base := DoerFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Body: http.NoBody}, nil
	})
	m := CircuitBreakerMiddleware(CircuitBreakerConfig{FailureThreshold: 2})
	composed := Apply(base, []Middleware{m}, nil)
	req, _ := http.NewRequest("GET", "http://client-err-host/a", nil)
	for i := 0; i < 5; i++ {
		composed.Do(req)
	}
	cb := getCircuitBreaker("http://client-err-host/a", nil)
	assert.Equal(t, "CLOSED", cb.State())
}

func TestCircuitBreakerMiddleware_recordsTransportErrorAsFailure(t *testing.T) {
	cbRegistry.Delete("http://transport-err-host")
	base := DoerFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})
	m := CircuitBreakerMiddleware(CircuitBreakerConfig{FailureThreshold: 1})
	composed := Apply(base, []Middleware{m}, nil)
	req, _ := http.NewRequest("GET", "http://transport-err-host/a", nil)
	composed.Do(req)

	cb := getCircuitBreaker("http://transport-err-host/a", nil)
	assert.Equal(t, "OPEN", cb.State())
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```
go test ./httpclient/... -run "TestCircuitBreaker" -v
```
Expected: `undefined: CircuitBreakerMiddleware`

- [ ] **Step 3: Append CircuitBreakerMiddleware to circuit_breaker.go**

Add `"net/http"` to the import block in `circuit_breaker.go`, then append after the `State()` method:

```go
// CircuitBreakerMiddleware wraps each request with circuit-breaker protection.
// State is global per host (keyed by scheme://host in cbRegistry sync.Map).
// cfg is applied only when a circuit breaker is first created for a host.
// 5xx responses and transport errors are recorded as failures; 4xx and 2xx as successes.
func CircuitBreakerMiddleware(cfg CircuitBreakerConfig) Middleware {
	return NewMiddleware(CircuitBreakerKey, func(next Doer) Doer {
		return DoerFunc(func(req *http.Request) (*http.Response, error) {
			cb := getCircuitBreaker(req.URL.String(), &cfg)
			if err := cb.Allow(); err != nil {
				return nil, err
			}
			resp, err := next.Do(req)
			if err != nil {
				cb.RecordFailure()
				return nil, err
			}
			if resp.StatusCode >= 500 {
				cb.RecordFailure()
			} else {
				cb.RecordSuccess()
			}
			return resp, nil
		})
	})
}
```

- [ ] **Step 4: Run all CB tests — expect all pass**

```
go test ./httpclient/... -run "TestCircuitBreaker" -v
```
Expected: all pass

- [ ] **Step 5: Commit**

```
git add httpclient/circuit_breaker.go httpclient/circuit_breaker_test.go
git commit -m "feat(httpclient): add CircuitBreakerMiddleware wrapping existing CB logic"
```

---

## Task 10: SSE Support

**Files:**
- Create: `httpclient/sse.go`
- Create: `httpclient/sse_test.go`

- [ ] **Step 1: Write failing tests**

```go
// httpclient/sse_test.go
package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sseServer(t *testing.T, events []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "ResponseWriter must implement http.Flusher")
		for _, e := range events {
			fmt.Fprint(w, e)
			flusher.Flush()
		}
	}))
}

func TestExecuteSSE_receivesEventsWithCorrectFields(t *testing.T) {
	srv := sseServer(t, []string{
		"event: update\ndata: hello\n\n",
		"data: world\n\n",
	})
	defer srv.Close()

	var received []SSEEvent
	err := New().Get(srv.URL).ExecuteSSE(func(e SSEEvent) error {
		received = append(received, e)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, received, 2)
	assert.Equal(t, "update", received[0].Event)
	assert.Equal(t, "hello", received[0].Data)
	assert.Equal(t, "message", received[1].Event) // default when no event field
	assert.Equal(t, "world", received[1].Data)
}

func TestExecuteSSE_callbackErrorStopsStream(t *testing.T) {
	srv := sseServer(t, []string{
		"data: first\n\n",
		"data: second\n\n",
	})
	defer srv.Close()

	count := 0
	err := New().Get(srv.URL).ExecuteSSE(func(e SSEEvent) error {
		count++
		return fmt.Errorf("stop after first")
	})
	assert.Error(t, err)
	assert.Equal(t, 1, count)
}

func TestExecuteSSE_contextCancellationStopsStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			fmt.Fprintf(w, "data: msg%d\n\n", i)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	count := 0
	err := New().Get(srv.URL).WithContext(ctx).ExecuteSSE(func(e SSEEvent) error {
		count++
		return nil
	})
	assert.Error(t, err)
	assert.Less(t, count, 100)
}

func TestParseSSELine_parsesAllFieldTypes(t *testing.T) {
	tests := []struct {
		input   string
		wantKey string
		wantVal string
	}{
		{"data: hello", "data", "hello"},
		{"event: update", "event", "update"},
		{"id: 42", "id", "42"},
		{"retry: 3000", "retry", "3000"},
		{": comment", "", "comment"},
		{"", "", ""},
		{"data:", "data", ""},
	}
	for _, tt := range tests {
		key, val := parseSSELine(tt.input)
		assert.Equal(t, tt.wantKey, key, "input: %q", tt.input)
		assert.Equal(t, tt.wantVal, val, "input: %q", tt.input)
	}
}

func TestSSEStream_defaultEventNameIsMessage(t *testing.T) {
	r := strings.NewReader("data: test\n\n")
	var events []SSEEvent
	parseSSEStream(r, func(e SSEEvent) error {
		events = append(events, e)
		return nil
	})
	require.Len(t, events, 1)
	assert.Equal(t, "message", events[0].Event)
	assert.Equal(t, "test", events[0].Data)
}

func TestSSEStream_multilineData_joinedWithNewline(t *testing.T) {
	r := strings.NewReader("data: line1\ndata: line2\n\n")
	var events []SSEEvent
	parseSSEStream(r, func(e SSEEvent) error {
		events = append(events, e)
		return nil
	})
	require.Len(t, events, 1)
	assert.Equal(t, "line1\nline2", events[0].Data)
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```
go test ./httpclient/... -run "TestExecuteSSE|TestParseSSE|TestSSEStream" -v
```
Expected: `undefined: SSEEvent`

- [ ] **Step 3: Write implementation**

```go
// httpclient/sse.go
package httpclient

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// SSEEvent represents a single Server-Sent Event as defined by RFC 8895.
type SSEEvent struct {
	ID    string
	Event string // defaults to "message" when the server omits the event field
	Data  string
	Retry int // reconnect hint from server in milliseconds (0 = not set)
}

// ExecuteSSE sends the request and reads the response as a text/event-stream.
// callback is invoked for each complete event; returning a non-nil error stops the stream.
// Blocks until the stream ends, callback errors, or ctx is cancelled.
func (rb *RequestBuilder) ExecuteSSE(callback func(SSEEvent) error) error {
	rb.applyDefaults()

	rawURL := rb.buildURL()
	ctx := rb.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, rb.method.String(), rawURL, nil)
	if err != nil {
		return fmt.Errorf("sse: build request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for k, vals := range rb.headers {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	hc := rb.client.buildHTTPClient(rb.skipTLS, rb.timeout)
	doer := Apply(hc, rb.client.middlewares, rb.skipMiddleware)

	resp, err := doer.Do(req)
	if err != nil {
		return fmt.Errorf("sse: send request: %w", err)
	}
	defer resp.Body.Close()

	return parseSSEStream(resp.Body, callback)
}

// parseSSEStream reads an SSE body line by line and calls callback for each complete event.
func parseSSEStream(r io.Reader, callback func(SSEEvent) error) error {
	scanner := bufio.NewScanner(r)
	var current SSEEvent
	current.Event = "message"

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// Empty line = event boundary
			if current.Data != "" {
				if err := callback(current); err != nil {
					return err
				}
			}
			current = SSEEvent{Event: "message"}
			continue
		}
		key, val := parseSSELine(line)
		switch key {
		case "data":
			if current.Data != "" {
				current.Data += "\n"
			}
			current.Data += val
		case "event":
			current.Event = val
		case "id":
			current.ID = val
		case "retry":
			if ms, err := strconv.Atoi(val); err == nil {
				current.Retry = ms
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("sse: read stream: %w", err)
	}
	return nil
}

// parseSSELine splits one SSE line into key and value.
// Comment lines (": ...") return ("", comment).
// Empty lines return ("", "").
func parseSSELine(line string) (key, value string) {
	if line == "" {
		return "", ""
	}
	if strings.HasPrefix(line, ":") {
		return "", strings.TrimPrefix(line, ": ")
	}
	idx := strings.Index(line, ":")
	if idx < 0 {
		return line, ""
	}
	key = line[:idx]
	value = strings.TrimPrefix(line[idx+1:], " ")
	return key, value
}
```

- [ ] **Step 4: Run SSE tests — expect all pass**

```
go test ./httpclient/... -run "TestExecuteSSE|TestParseSSE|TestSSEStream" -v
```
Expected: all pass

- [ ] **Step 5: Commit**

```
git add httpclient/sse.go httpclient/sse_test.go
git commit -m "feat(httpclient): add SSE support with ExecuteSSE() and RFC 8895 line parser"
```

---

## Task 11: Migration & Cleanup

**Files:**
- Delete: `httpstd/` entire package
- Delete: old resty references
- Modify: `go.mod` — remove resty
- Update: `CLAUDE.md`, `README.MD`

- [ ] **Step 1: Delete httpstd package**

```
git rm -r httpstd/
```

- [ ] **Step 2: Verify no remaining resty imports**

```
grep -r "go-resty" --include="*.go" .
```
Expected: no output. If any remain, remove them.

- [ ] **Step 3: Run go mod tidy**

```
go mod tidy
grep "go-resty" go.mod
```
Expected: no output (resty removed from go.mod)

- [ ] **Step 4: Run full test suite**

```
go test ./... -count=1
```
Expected: all packages pass

- [ ] **Step 5: Update CLAUDE.md — replace "HTTP Client Design" section**

Replace the existing `### HTTP Client Design` section with:

```markdown
### HTTP Client Design (`httpclient`)

Single package `httpclient` (stdlib only — no resty or other HTTP library dependencies).

**Core pattern:**
```go
client := httpclient.New(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithDefaultTimeout(10 * time.Second),
    httpclient.WithMiddleware(httpclient.LoggingMiddleware(log)),
    httpclient.WithMiddleware(httpclient.RetryMiddleware(httpclient.RetryConfig{
        MaxAttempts: 3, Backoff: httpclient.ExponentialBackoff(200*time.Millisecond, 2.0),
        RetryOn: httpclient.RetryOnAny,
    })),
    httpclient.WithMiddleware(httpclient.CircuitBreakerMiddleware(httpclient.CircuitBreakerConfig{})),
)

var result MyStruct
err := client.Post("/users").WithBody(payload).WithBearer(token).Execute().Consume(&result)
```

**Middleware chain:** `Doer` interface + `Middleware` type. `Apply(base, middlewares, skip)` composes right-to-left; first middleware registered is outermost. Retry, circuit breaker, logging are all middleware — not flags on the request struct.

**Package-level shortcuts** (backward-compatible, use global client with no middleware):
```go
httpclient.Post("https://api.example.com/users").WithBody(x).Execute()
```

**Request types:** JSON (default `WithBody`), form (`WithForm`), multipart (`WithMultipart`), raw bytes (`WithRawBody`), GraphQL sugar (`WithGraphQL`).

**SSE:** `ExecuteSSE(func(SSEEvent) error)` reads `text/event-stream` via `bufio.Scanner`.

**Streaming download:** `WithOutput(w io.Writer)` — `Response.Body` is nil after streaming; never call `Consume` or `SaveToFile` on the same response.

**Response headers:** `resp.Headers http.Header` — always populated on non-streaming responses.

**Per-request middleware skip:** `WithoutMiddleware(CircuitBreakerKey)` rebuilds stack without that middleware.

**`WithQueryParam`** is kept as an alias for `WithQueryParams` (backward compat).

**CB state is global per host** (`sync.Map` keyed by `scheme://host`) — same logic as before, now as middleware.
```

- [ ] **Step 6: Update README.MD httpclient section**

Find the httpclient section in README.MD and update to:
1. Remove any `httpstd` references
2. Show the `New(opts...)` pattern first
3. Show all new `With*` methods in a table or examples

- [ ] **Step 7: Full build**

```
go build ./...
```
Expected: success

- [ ] **Step 8: Final test run**

```
go test ./... -count=1 -v 2>&1 | grep -E "^(ok|FAIL|---)"
```
Expected: all `ok`, zero `FAIL`

- [ ] **Step 9: Commit**

```
git add -A
git commit -m "feat(httpclient): complete migration — remove httpstd, remove resty, update docs

- Delete httpstd/ package (replaced by unified httpclient)
- Remove go-resty/resty from go.mod
- Update CLAUDE.md HTTP Client Design section
- Update README.MD examples

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- [x] `Doer`/`DoerFunc`/`Middleware`/`Apply` — Task 1
- [x] `LoggingMiddleware` — Task 1 (combined with middleware core)
- [x] `Response` with `Headers`, `ConsumeXML`, `ConsumeText`, `Raw()` — Task 2
- [x] `Client` with all `ClientOption` funcs — Task 3
- [x] `RequestBuilder` with all `With*` methods including `WithGraphQL`, `WithCookie`, `WithoutMiddleware` — Task 4
- [x] `buildBody` (JSON, form, multipart, raw, GraphQL via JSON) — Task 5
- [x] `buildURL` with base URL + path params — Task 5
- [x] `Execute()` with middleware stack, streaming, context, cookies, default auth — Task 6
- [x] Package-level backward-compatible shortcuts — Task 7
- [x] Retry with all backoff strategies, `RetryOn*` helpers, context cancellation — Task 8
- [x] Circuit breaker as `Middleware` type — Task 9
- [x] SSE with `ExecuteSSE`, `SSEEvent`, RFC 8895 parser, multiline data — Task 10
- [x] `httpstd` deletion, resty removal, CLAUDE.md + README update — Task 11

**Type consistency across tasks:**
- `Middleware` struct with unexported `key`/`fn` + `NewMiddleware` constructor — Tasks 1, 8, 9
- `CircuitBreakerConfig` (existing name, kept) — Tasks 9, 11
- `RetryConfig.RetryOn` is `func(*http.Response, error) bool` — consistent Tasks 8 tests + impl
- `SSEEvent{ID, Event, Data string; Retry int}` — consistent Tasks 10 tests + impl
- `buildURL()`, `buildBody()`, `applyDefaults()` are `*RequestBuilder` methods — Tasks 5→6
- `Response.raw` field (unexported) set in `Execute()`, exposed via `Raw()` — Tasks 2, 6
- `WithQueryParam` as alias for `WithQueryParams` — Tasks 4, 11

**Breaking changes documented:**
- `httpstd` package deleted → migrate import to `httpclient`
- `DisableCircuitBreaker bool` field → `WithoutMiddleware(CircuitBreakerKey)`
- `WithFile([]MultipartData)` → `WithMultipart(Field(...), FileFromReader(...))`
- `WithoutCircuitBreaker()` method → `WithoutMiddleware(CircuitBreakerKey)`
