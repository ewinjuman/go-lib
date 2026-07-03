# go-lib/v2

A production-ready Go utility library for building scalable web services and microservices. Provides structured logging, HTTP/gRPC clients, error handling, request context management, and various utilities — all with a consistent, ergonomic API.

**Module:** `github.com/ewinjuman/go-lib/v2`  
**Go Version:** 1.21+

---

## Installation

```bash
go get github.com/ewinjuman/go-lib/v2
```

---

## Packages

| Package | Description |
|---|---|
| [`logger`](#logger) | Structured async logging with masking and file rotation |
| [`appContext`](#appcontext) | Request-scoped context for distributed tracing |
| [`httpclient`](#http-client) | Fluent HTTP client (stdlib) with middleware, circuit breaker, retry |
| [`grpc`](#grpc-client) | gRPC client with automatic context and metadata |
| [`apperror`](#error-handling) | Standardized error types with HTTP/gRPC code mapping |
| [`cache`](#cache) | Redis client (standalone/Sentinel/Cluster), typed `Store[T]`, stampede-protected `ObjectCache[T]`, distributed `Lock` |
| [`database`](#database) | GORM connection factory for MySQL/PostgreSQL, wired with `logger` |
| [`password`](#password) | Bcrypt password hashing and comparison |
| [`constant`](#constant) | Context key constants for request propagation |
| [`utils`](#utils) | General utilities: conversion, retry, code generation |

---

## Logger

Structured, async logger built on [zap](https://github.com/uber-go/zap) with support for file rotation, data masking, redaction, and multiple outputs.

### Initialization

```go
import "github.com/ewinjuman/go-lib/v2/logger"

opts := logger.Options{
    AppName:     "my-service",
    Environment: "production",
    Stdout:      true,           // print to console
    Write:       true,           // write to file
    Filename:    "logs/app.log", // active file: logs/app-2026-05-25.log (date appended automatically)
    MaxSize:     100,            // MB — size-based rotation within a day
    MaxBackups:  7,
    MaxAge:      7,              // days to keep rotated backup files
    Level:       logger.InfoLevel,
}

log, err := logger.New(opts)
if err != nil {
    panic(err)
}
defer log.Shutdown() // safe to call multiple times (idempotent)
```

Or use the global singleton:

```go
logger.InitLogger(opts)
log := logger.GetLogger()
```

### Basic Logging

All log methods take `context.Context` as first argument so trace/request IDs are automatically included:

```go
ctx := context.Background()

log.Info(ctx, "user created", logger.String("user_id", "u-123"))
log.Warn(ctx, "rate limit approaching", logger.Int("remaining", 5))
log.Error(ctx, "payment failed", logger.Error(err), logger.String("order_id", "o-456"))
log.Debug(ctx, "cache miss", logger.String("key", "product:42"))
```

### WithContext — bind context once

Use `log.WithContext(ctx)` to get a `*ContextualLogger` with the context already bound.
Subsequent calls do **not** need to pass `ctx`:

```go
clog := logger.GetLogger().WithContext(ctx)

clog.Info("user created", logger.String("user_id", "u-123"))
clog.Warn("rate limit approaching", logger.Int("remaining", 5))
clog.Error("payment failed", logger.Error(err))

// Switch to a different context — returns a new ContextualLogger:
reqLog := clog.WithContext(requestCtx)
reqLog.Info("handling request")
```

This is equivalent to `appCtx.Log()` in Fiber handlers — the same `ContextualLogger` type,
just obtained directly from a `*Logger` instead of from an `AppContext`.

### Field Helpers

```go
logger.String("key", "value")
logger.Int("count", 10)
logger.Int64("timestamp", time.Now().Unix())
logger.Float64("ratio", 0.95)
logger.Bool("enabled", true)
logger.Duration("elapsed", 250*time.Millisecond)
logger.Interface("payload", myStruct)
logger.Error(err)
```

### Data Masking & Redaction

Sensitive fields in logged structs or JSON are automatically masked or fully redacted.

```go
opts := logger.Options{
    MaskingPaths:   []string{"credit_card", "token", "email"},  // partially masked
    RedactionPaths: []string{"password", "pin"},                 // replaced with [REDACTED]
}
```

Masking example: `"email": "u***@***.com"` (domain masked, only TLD shown), `"token": "ab****xyz"`

> **Async log timing:** Each log entry includes a `logged_at` field capturing the exact time the caller invoked `Info`/`Debug`/etc. The `timestamp` field records when the async worker processed the entry — typically within `FlushInterval` (500ms default).

### HTTP / gRPC Request Logging

```go
log.LogRequestHttp(ctx, url, method, body, header, params)
log.LogResponseHttp(ctx, elapsed, statusCode, url, method, body, err)

log.LogRequestGrpc(ctx, method, "GRPC", requestBody, metadata)
log.LogResponseGrpc(ctx, startTime, method, "GRPC", responseBody)
```

### GORM Database Logging

```go
import (
    "github.com/ewinjuman/go-lib/v2/logger"
    gormlogger "gorm.io/gorm/logger"
)

gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    Logger: logger.NewGormLogger(log, gormlogger.Config{
        SlowThreshold:             time.Second,
        LogLevel:                  gormlogger.Info,
        IgnoreRecordNotFoundError: false,
    }),
})
```

### Options Reference

| Field | Type | Default | Description |
|---|---|---|---|
| `AppName` | string | `"app"` | Service identifier added to every log |
| `Environment` | string | `"development"` | Environment tag (dev/staging/prod) |
| `Stdout` | bool | `true` | Print to console |
| `Write` | bool | `false` | Write to file |
| `Filename` | string | `""` | Base log file path — date is appended automatically (e.g. `logs/app-2026-05-25.log`). A new file is created each day; process restart not required. |
| `MaxSize` | int | `100` | Max file size in MB before size-based rotation within a day |
| `MaxBackups` | int | `7` | Number of rotated backup files to keep |
| `MaxAge` | int | `24` | Max age in **days** to retain rotated backup files |
| `Compress` | bool | `true` | Compress rotated log files |
| `Level` | Level | `InfoLevel` | Minimum log level |
| `MaskingPaths` | []string | `nil` | Fields to partially mask |
| `RedactionPaths` | []string | `nil` | Fields to fully redact |
| `DefaultFields` | map[string]string | `nil` | Fields added to every log entry |
| `EnableTrace` | bool | `false` | Attach stack traces to error logs |
| `Development` | bool | `false` | Pretty console output |
| `BufferSize` | int | `256` | Async log buffer size |
| `FlushInterval` | Duration | `500ms` | Auto-flush interval |
| `WorkerPoolSize` | int | `2` | Number of concurrent async log workers |
| `DisableAsync` | bool | `false` | Disable async buffering (synchronous mode) |
| `DisableMasking` | bool | `false` | Disable masking globally |

---

## AppContext

Request-scoped context for propagating request metadata (trace IDs, user IDs, IP, etc.) across service layers.

All fields are **unexported** — use the provided `Set*` / `Get*` accessor methods. `ToContext()` result is cached and only rebuilt when a propagated field changes, so calling `Log()` many times per request is cheap.

### Creating a Context

`New` takes a parent `context.Context` as first argument so deadlines and cancellations from the HTTP request are preserved. The logger is optional — if `nil`, the global singleton is used.

```go
import (
    "context"
    "github.com/ewinjuman/go-lib/v2/appContext"
    "github.com/ewinjuman/go-lib/v2/logger"
)

log := logger.GetLogger()

// With explicit parent context (recommended in handlers)
appCtx := appContext.New(ctx, log).
    SetTraceID("trace-abc-123").
    SetUserID("user-456").
    SetIP("192.168.1.1").
    SetMethod("POST").
    SetURL("/api/orders")

// With nil logger — falls back to global singleton
appCtx = appContext.New(ctx, nil)
```

### From Fiber Request

```go
func handler(c *fiber.Ctx) error {
    // FromFiber never returns nil — falls back to New(c.UserContext(), nil)
    appCtx := appContext.FromFiber(c)
    appCtx.Log().Info("handling request")
    return nil
}
```

In middleware, create the AppContext with the Fiber request context so deadlines propagate:

```go
func contextMiddleware(c *fiber.Ctx) error {
    appCtx := appContext.New(c.UserContext(), log).
        SetRequestID(c.Get("X-Request-ID")).
        SetTenantID(c.Get("X-Tenant-ID")).
        SetIP(c.IP()).
        SetMethod(c.Method()).
        SetURL(c.Path())
    c.Locals(constant.AppContextKey, appCtx)
    return c.Next()
}

// After-middleware: log response info
func afterMiddleware(c *fiber.Ctx) error {
    err := c.Next()
    appCtx := appContext.FromFiber(c)
    appCtx.SetResponseStatus(c.Response().StatusCode())
    appCtx.Log().Info("request completed",
        logger.Int("status", appCtx.GetResponseStatus()),
        logger.Duration("elapsed", appCtx.Duration()),
    )
    return err
}
```

### From stdlib context (service layer)

AppContext is stored inside the context chain automatically when `ToContext()` is called. Any service layer that receives a `context.Context` can retrieve it:

```go
func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) error {
    appCtx := appContext.FromContext(ctx) // never panics — returns nil if not found
    if appCtx != nil {
        appCtx.Log().Info("creating user", logger.String("email", req.Email))
    }
    // ...
}
```

### Setters & Getters

Fields propagated to `context.Context` (available via `constant.*Key` in downstream calls):

| Setter | Getter | Context key |
|--------|--------|-------------|
| `SetRequestID(string)` | `GetRequestID() string` | `constant.RequestIDKey` |
| `SetTraceID(string)` | `GetTraceID() string` | `constant.TraceIDKey` |
| `SetUserID(string)` | `GetUserID() string` | `constant.UserIDKey` |
| `SetTenantID(string)` | `GetTenantID() string` | `constant.TenantIDKey` |
| `SetRequestTime(time.Time)` | `GetRequestTime() time.Time` | `constant.RequestTimeKey` |
| `SetMethod(string)` | `GetMethod() string` | `constant.RequestMethodKey` |
| `SetURL(string)` | `GetURL() string` | `constant.RequestPathKey` |
| `SetIP(string)` | `GetIP() string` | `constant.RequestIPKey` |
| `SetUserAgent(string)` | `GetUserAgent() string` | `constant.RequestAgentKey` |

Fields stored locally (not propagated to context):

| Setter | Getter | Purpose |
|--------|--------|---------|
| `SetResponseStatus(int)` | `GetResponseStatus() int` | HTTP response code (set in after-middleware) |
| `SetPort(int)` | `GetPort() int` | Listening port |
| `SetSrcIP(string)` | `GetSrcIP() string` | Source IP before proxying |
| `SetHeader(interface{})` | `GetHeader() interface{}` | Raw request header |
| `SetRequest(interface{})` | `GetRequest() interface{}` | Raw request body |

All setters are chainable and safe for concurrent use.

### Logging via AppContext

`Log()` returns a `*ContextualLogger` already bound to the request context — no need to pass `ToContext()` manually:

```go
appCtx.Log().Info("order placed", logger.String("order_id", "o-789"))
appCtx.Log().Error("failed to charge card", logger.Error(err))
appCtx.Log().Warn("retry attempt", logger.Int("attempt", 2))
appCtx.Log().Debug("cache lookup", logger.String("key", "user:123"))

// HTTP / gRPC request logging
appCtx.Log().LogRequestHttp(url, method, body, headers, params)
appCtx.Log().LogResponseHttp(elapsed, statusCode, url, method, body, err)
appCtx.Log().LogRequestGrpc(method, "GRPC", requestBody, metadata)
appCtx.Log().LogResponseGrpc(startTime, method, "GRPC", responseBody)
```

### Custom Key-Value Storage

Thread-safe key-value store scoped to the request (backed by `sync.Map`):

```go
appCtx.Put("tenant_id", "tenant-001")
tenantID := appCtx.Get("tenant_id", "default") // second arg = default value
appCtx.Remove("tenant_id")
```

### Clone — for outgoing service calls

`Clone()` creates a new AppContext with a fresh `requestID` and `requestTime` while carrying over all propagated identity fields (`traceID`, `userID`, `tenantID`, etc.). Local-only fields and the key-value store are not copied.

```go
// Outgoing call to another microservice
outCtx := appCtx.Clone()
grpcCtx, cancel := conn.CreateContext(ctx, outCtx)
defer cancel()
resp, err := userClient.GetUser(grpcCtx, req)
```

### Propagating to stdlib context

```go
ctx := appCtx.ToContext()
// Propagated fields (trace_id, request_id, user_id, tenant_id, etc.) are now in ctx.
// The AppContext itself is also in ctx — retrieve it anywhere with FromContext(ctx).
// Result is cached — no allocations on repeated calls until a Set* changes a field.
```

### Duration

```go
// Elapsed time since request started — useful in after-middleware
appCtx.Duration() // time.Duration
```

---

## HTTP Client

Fluent HTTP client backed by stdlib `net/http` — no external HTTP library dependencies. Supports a middleware pipeline (logging, retry, circuit breaker) via a composable `Doer` interface.

### Creating a Client

```go
import "github.com/ewinjuman/go-lib/v2/httpclient"

// Client with middleware pipeline
client := httpclient.New(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithDefaultTimeout(10 * time.Second),
    httpclient.WithMiddleware(httpclient.LoggingMiddleware(log)),
    httpclient.WithMiddleware(httpclient.RetryMiddleware(httpclient.RetryConfig{
        MaxAttempts: 3,
        Backoff:     httpclient.ExponentialBackoff(200*time.Millisecond, 2.0),
        RetryOn:     httpclient.RetryOnAny,
    })),
    httpclient.WithMiddleware(httpclient.CircuitBreakerMiddleware(httpclient.CircuitBreakerConfig{})),
)

var result MyResponse
err := client.Get("/users").
    WithQueryParam(map[string]string{"page": "1", "limit": "20"}).
    Execute().
    Consume(&result)
```

### Package-Level Shortcuts (Backward Compatible)

```go
// Package-level functions use a global client with no middleware
import "github.com/ewinjuman/go-lib/v2/httpclient"

// GET
var result MyResponse
err := httpclient.Get("https://api.example.com/users").
    WithQueryParam(map[string]string{"page": "1", "limit": "20"}).
    Execute().
    Consume(&result)

// POST with JSON body
err = httpclient.Post("https://api.example.com/users").
    WithBody(CreateUserRequest{Name: "Alice", Email: "alice@example.com"}).
    WithHeaders(map[string]string{"X-API-Key": "my-api-key"}).
    Execute().
    Consume(&result)

// PUT with path parameter
err = httpclient.Put("https://api.example.com/users/:id").
    WithPathParam(map[string]string{"id": "u-123"}).
    WithBody(updatePayload).
    Execute().
    Consume(&result)

// DELETE
err = httpclient.Delete("https://api.example.com/users/:id").
    WithPathParam(map[string]string{"id": "u-123"}).
    Execute().
    Consume(&result)
```

### Authentication

```go
httpclient.Get("https://api.example.com/protected").
    WithBasicAuth("username", "password").
    Execute()

httpclient.Get("https://api.example.com/protected").
    WithBearer("my-token").
    Execute()
```

### Request Configuration

```go
httpclient.Post("https://api.example.com/orders").
    WithContext(appCtx.ToContext()).
    WithTimeout(30 * time.Second).
    WithRequestID("req-xyz").
    WithDebug(true).
    WithSkipTLS().
    WithBody(orderPayload).
    Execute().
    Consume(&result)
```

### Custom Success Codes

By default, only HTTP 200 is treated as success. Override per request:

```go
httpclient.Post("https://api.example.com/resources").
    WithBody(payload).
    WithSuccessCodes([]int{200, 201, 202}).
    Execute().
    Consume(&result)
```

`Consume`, `IsSuccess`, and `IsError` all respect the configured codes.

### Checking Response

```go
resp := httpclient.Get("https://api.example.com/status").Execute()

if resp.IsSuccess() {
    fmt.Println("HTTP", resp.HttpCode())
    resp.Consume(&myStruct)
}

if ok, err := resp.IsError(); ok {
    // err contains status code and body details
}
```

### Circuit Breaker

Circuit breaker is **on by default** per host (`scheme://host`). It opens after 5 failures or a 50% failure rate, and probes again after 1 minute.

```go
// Tune thresholds (applied on first request to the host)
httpclient.Post("https://api.payment.com/charge").
    WithBody(payload).
    WithCircuitBreakerConfig(httpclient.CircuitBreakerConfig{
        FailureThreshold:     3,
        RecoveryTimeout:      30 * time.Second,
        FailureRateThreshold: 0.3,
    }).
    Execute()

// Disable for a specific request
httpclient.Get("http://internal-svc/ping").
    WithoutCircuitBreaker().
    Execute()
```

HTTP 5xx responses count as failures; 4xx does not trip the breaker.

### Multipart File Upload

```go
file, _ := os.Open("/tmp/photo.jpg")
defer file.Close()

httpclient.Post("https://api.example.com/upload").
    WithFile([]httpclient.MultipartData{
        {Key: "file", Value: "photo.jpg", File: file},
    }).
    Execute()
```

### File Download

**Small files** — buffer in memory, then save to disk:

```go
err := httpclient.Get("https://api.example.com/reports/123.pdf").
    WithBearer("my-token").
    Execute().
    SaveToFile("report.pdf")
```

**Large files** — stream directly to disk without buffering:

```go
f, err := os.Create("video.mp4")
if err != nil {
    return err
}
defer f.Close()

resp := httpclient.Get("https://cdn.example.com/video.mp4").
    WithOutput(f).
    WithTimeout(5 * time.Minute).
    Execute()

if _, err := resp.IsError(); err != nil {
    return err
}
```

`WithOutput` streams the response body directly to any `io.Writer` (file, buffer, pipe, etc.). `Response.Body` will be `nil` after streaming — do not call `Consume` or `SaveToFile` on the same response.

---

## gRPC Client

Lightweight gRPC client wrapper with context propagation and automatic request/response logging.

### Creating a Connection

```go
import "github.com/ewinjuman/go-lib/v2/grpc"

conn, err := grpc.New(grpc.Options{
    Address: "user-service:5000",
    Timeout: 30,
})
if err != nil {
    log.Fatal(err)
}
```

### Creating a Context for RPC Calls

```go
// CreateContext adds timeout and propagates request ID to outgoing metadata
ctx, cancel := conn.CreateContext(context.Background(), appCtx)
defer cancel()

resp, err := userClient.GetUser(ctx, &pb.GetUserRequest{Id: "u-123"})
```

---

## Error Handling

Standardized error types that map cleanly between HTTP status codes and gRPC codes.

### Named Constructors

Preferred way to create errors — returns `*ApplicationError` so you can chain `.WithCause` without a type assertion:

```go
import "github.com/ewinjuman/go-lib/v2/apperror"

// Default message comes from the HTTP standard ("Not Found", "Bad Request", etc.)
err := apperror.NotFound()
err  = apperror.BadRequest()
err  = apperror.Unauthorized()
err  = apperror.Forbidden()
err  = apperror.Conflict()
err  = apperror.UnprocessableEntity()
err  = apperror.TooManyRequests()
err  = apperror.InternalError()
err  = apperror.ServiceUnavailable()

// Custom message
err = apperror.NotFound("user id=42 not found")

// Chain a root cause
err = apperror.NotFound("user not found").WithCause(sql.ErrNoRows)

// Retrieve the root cause
if ae, ok := err.(*apperror.ApplicationError); ok {
    fmt.Println(ae.Cause()) // sql.ErrNoRows
}
```

### Sentinel Errors & errors.Is

Named sentinels let you use `errors.Is` for type-safe checks, even through wrapped errors:

```go
// Sentinel variables — compare by ErrorCode + Status
apperror.ErrNotFound
apperror.ErrBadRequest
apperror.ErrUnauthorized
apperror.ErrForbidden
apperror.ErrConflict
apperror.ErrUnprocessableEntity
apperror.ErrTooManyRequests
apperror.ErrInternalError
apperror.ErrServiceUnavailable

// errors.Is works through fmt.Errorf wrapping
err := fmt.Errorf("service layer: %w", apperror.NotFound("item missing"))
errors.Is(err, apperror.ErrNotFound) // true
```

### Named Predicates

Shorthand for `errors.Is` — works with wrapped errors:

```go
apperror.IsNotFound(err)          // true if err is/wraps a 404
apperror.IsBadRequest(err)        // 400
apperror.IsUnauthorized(err)      // 401
apperror.IsForbidden(err)         // 403
apperror.IsConflict(err)          // 409
apperror.IsUnprocessable(err)     // 422
apperror.IsTooManyRequests(err)   // 429
apperror.IsInternalError(err)     // 500
apperror.IsServiceUnavailable(err) // 503
```

### Low-level Constructors

For custom status strings or non-standard codes:

```go
err := apperror.New(400, "VALIDATION_ERROR", "invalid email format")
err  = apperror.NewError(404, "NOT_FOUND")
err  = apperror.NewError(422, "UNPROCESSABLE", "email already exists")
err  = apperror.DeadlineExceededError("upstream API timed out")
```

### Inspecting Errors

```go
code := apperror.GetCode(err)      // HTTP status code, defaults to 500

if apperror.IsTimeout(err) { ... } // handles os timeout + gRPC DeadlineExceeded

appErr := apperror.ParseError(err) // works on gRPC, ApplicationError, and plain errors
fmt.Println(appErr.ErrorCode, appErr.Status, appErr.Message)

text := apperror.StatusMessage(422) // "Unprocessable Entity"
```

### Converting to gRPC Status

```go
appErr := apperror.NotFound("user not found")
st := appErr.ToGRPCStatus()     // *status.Status
return nil, st.Err()            // return from a gRPC handler
```

HTTP codes are mapped to the nearest gRPC code (404 → `NotFound`, 401 → `Unauthenticated`, 429 → `ResourceExhausted`, 504 → `DeadlineExceeded`, etc.). Unmapped codes fall back to `codes.Unknown`.

### ApplicationError Type

```go
type ApplicationError struct {
    ErrorCode int    // HTTP status code
    Status    string // e.g. "NOT_FOUND"
    Message   string // human-readable message
    // cause is unexported — access via .Cause() and .Unwrap()
}

// Key methods:
func (e *ApplicationError) Error() string
func (e *ApplicationError) Unwrap() error                       // errors chain traversal
func (e *ApplicationError) Cause() error                        // direct cause access
func (e *ApplicationError) WithCause(err error) *ApplicationError // returns new instance — original unchanged
func (e *ApplicationError) Is(target error) bool                // matches by ErrorCode + Status
func (e *ApplicationError) ToGRPCStatus() *status.Status        // convert to gRPC status
```

---

## Cache

Generic Redis caching toolkit: a `RedisClient` wrapper that works identically over a standalone, Sentinel, or Cluster connection; a typed `Store[T]`; a stampede-protected `ObjectCache[T]`; and a distributed `Lock`.

```go
import "github.com/ewinjuman/go-lib/v2/cache"

client, err := cache.NewRedisClient(cache.RedisOption{
    Address:  "localhost:6379", // single node
    Password: "",
    Database: 0,
    PoolSize: 10,
})

// Or wrap an already-connected redis.UniversalClient instead of dialing a new one:
client := cache.WrapRedisClient(existingUniversalClient)
```

### Standalone, Sentinel, or Cluster

The concrete client type is selected from `RedisOption` — no separate constructors needed:

```go
type RedisOption struct {
    Address       string        // single host:port
    Addresses     []string      // cluster/sentinel seed nodes (takes precedence over Address)
    Username      string
    Password      string
    Database      int           // ignored in cluster mode
    PoolSize      int
    MasterName    string        // set to enable Sentinel-backed failover
    IsClusterMode bool          // force cluster mode with a single address (e.g. AWS ElastiCache)
    MaxRetries    int           // 0 = go-redis default (3); -1 disables retries
    DialTimeout   time.Duration // 0 = go-redis default (5s)
    ReadTimeout   time.Duration // 0 = go-redis default (3s)
    WriteTimeout  time.Duration // 0 = go-redis default (3s)
}
```

- `len(Addresses) > 1` or `IsClusterMode: true` → cluster client
- `MasterName` set → Sentinel-backed failover client
- otherwise → standalone client

`RedisClient` methods: `Get`, `Set`, `Delete`, `SetNX`, `Pipeline`, `Reset` (FlushAll), `Close`, `IsAlive`, `GetClient() redis.UniversalClient`, `TryLock`.

### Typed Store

```go
store := cache.NewStore[User](client, 10*time.Minute)
_ = store.Set(ctx, "user:123", &user)
got, err := store.Get(ctx, "user:123")
```

### Object Cache (stampede-protected)

```go
oc := cache.NewObjectCache[User](client, cache.ObjectCacheConfig{
    TTL:    10 * time.Minute,
    Policy: cache.PolicyAll(), // or PolicyNone(), PolicyKeys(...), PolicyFunc(...)
})

user, err := oc.GetOrLoad(ctx, "user:123", func() (*User, error) {
    return repo.FindUser(ctx, 123) // called at most once across concurrent callers
})
```

Concurrent cache misses for the same key are coalesced two ways: an in-process `singleflight.Group` first, then a Redis distributed lock across pods. Callers that lose the lock poll the cache for up to `LockWait` before falling back to a direct (uncached) load — never blocking indefinitely.

### Distributed Lock

```go
lock, ok, err := client.TryLock(ctx, "job:daily-report", 30*time.Second)
if ok {
    defer lock.Unlock(ctx)
    // do work
}
```

`Unlock` only releases the lock if the caller still holds it (token-based Lua check-and-delete), so an expired lock reacquired by someone else is never released out from under them.

---

## Database

GORM connection factory for MySQL or PostgreSQL, wired with `logger` for query logging.

```go
import "github.com/ewinjuman/go-lib/v2/database"

db, err := database.NewConnection(database.Option{
    DbType:      "postgres", // or "mysql"; empty defaults to postgres
    Host:        "localhost",
    Port:        5432,
    Username:    "user",
    Password:    "secret",
    Schema:      "app_db",
    MaxIdleConn: 10,
    MaxOpenConn: 100,
    LogMode:     true, // routes GORM query logs through the given *logger.Logger
    SslMode:     false,
    Timezone:    "Asia/Jakarta", // defaults to Asia/Jakarta when empty
}, log)
```

`logger` may be `nil` only when `LogMode: false` — GORM's `Info`/`Warn`/`Error` callbacks call it directly and will panic on a nil logger if `LogMode: true`.

---

## Password

Bcrypt-based password hashing utilities.

```go
import "github.com/ewinjuman/go-lib/v2/password"

hash, err := password.GeneratePassword("user_secret_password")

ok := password.ComparePasswords(storedHash, inputPassword)
if !ok {
    // return unauthorized
}
```

---

## Constant

Context key constants for consistent key-value access across the request lifecycle.

```go
import "github.com/ewinjuman/go-lib/v2/constant"

constant.AppContextKey     // AppContext stored in fiber.Locals
constant.RequestIDKey
constant.TraceIDKey
constant.UserIDKey
constant.RequestTimeKey
constant.RequestMethodKey
constant.RequestPathKey
constant.RequestIPKey
constant.RequestAgentKey
constant.ResponseStatusKey
constant.ResponseTimeKey

// Retrieve from stdlib context
requestID := ctx.Value(constant.RequestIDKey)
```

---

## Utils

### Retry

```go
import "github.com/ewinjuman/go-lib/v2/utils/retry"

// 3 attempts, incremental backoff (100ms → 200ms → 300ms)
err := retry.Do(ctx, func() error {
    return callExternalAPI()
})

// 3 attempts, jitter backoff (100ms → 200ms → 500ms)
err = retry.DoWithCustom(ctx, func() error {
    return callExternalAPI()
})

// Typed response variant
result, err := retry.DoWithResponse(ctx, func() (*MyResult, error) {
    return fetchData()
})

if retry.ShouldRetry(err) { ... }
```

### Convert

```go
import "github.com/ewinjuman/go-lib/v2/utils/convert"

var dst TargetType
convert.ObjectToObject(src, &dst)

jsonStr := convert.ObjectToString(myStruct)
convert.StringToObject(jsonStr, &myStruct)
convert.ByteToObject(jsonBytes, &myStruct)

convert.ConvertToWord(1500)              // "seribu lima ratus"
convert.ConvertEng(1500)                 // "one thousand five hundred"
convert.ConvertEngAnd(1500)              // "one thousand and five hundred"
convert.ConvertDateID(time.Now(), "2 January 2006") // "13 Mei 2026"
```

### Code Generator

```go
import "github.com/ewinjuman/go-lib/v2/utils/codeGenerator"

codeGenerator.RandomString(16, codeGenerator.AlphanumericCharset)
codeGenerator.RandomString(6,  codeGenerator.NumbersCharset)
codeGenerator.RandomString(8,  codeGenerator.LowerCaseLettersCharset)
```

Available charsets: `LowerCaseLettersCharset`, `UpperCaseLettersCharset`, `NumbersCharset`, `AlphanumericCharset`, `SpecialCharset`, `AllCharset`, `AlphanumericLowerCaseCharset`, `AlphanumericUpperCaseCharset`.

### General Utilities

```go
import "github.com/ewinjuman/go-lib/v2/utils"

utils.ContainsInArr([]string{"a", "b"}, "b")
utils.ContainsInArrNoCaseSensitive(slice, "Admin")
utils.BetweenString("hello world foo", "hello ", " foo") // "world"
utils.BeforeString("user@example.com", "@")              // "user"
utils.AfterString("user@example.com", "@")               // "example.com"
utils.Or("", "fallback")                                 // "fallback"
utils.Must(loadConfig())                                 // panics on error
utils.SafeDeref(maybeNilPtr)                             // zero value if nil
```

---

## Full Example

```go
package main

import (
    "context"

    appctx "github.com/ewinjuman/go-lib/v2/appContext"
    "github.com/ewinjuman/go-lib/v2/apperror"
    "github.com/ewinjuman/go-lib/v2/httpclient"
    "github.com/ewinjuman/go-lib/v2/logger"
    "github.com/ewinjuman/go-lib/v2/password"
)

func main() {
    // 1. Initialize logger
    logger.InitLogger(logger.Options{
        AppName:        "my-service",
        Environment:    "production",
        Stdout:         true,
        Write:          true,
        Filename:       "logs/app.log",
        Level:          logger.InfoLevel,
        MaskingPaths:   []string{"email", "token"},
        RedactionPaths: []string{"password"},
    })
    log := logger.GetLogger()
    defer log.Shutdown()

    // 2. Create request context — parent ctx carries deadline/cancellation
    appCtx := appctx.New(context.Background(), log).
        SetTraceID("trace-abc").
        SetUserID("user-123")

    appCtx.Log().Info("starting request")

    // 3. Hash password
    hash, _ := password.GeneratePassword("secret")
    if !password.ComparePasswords(hash, "secret") {
        appCtx.Log().Error("password mismatch")
        return
    }

    // 4. Call external API
    type APIResponse struct {
        ID   string `json:"id"`
        Name string `json:"name"`
    }

    var apiResp APIResponse
    err := httpclient.Get("https://api.example.com/users/:id").
        WithContext(appCtx.ToContext()).
        WithPathParam(map[string]string{"id": "u-123"}).
        WithBearer("my-token").
        Execute().
        Consume(&apiResp)

    if err != nil {
        appErr := apperror.ParseError(err)
        appCtx.Log().Error("API call failed",
            logger.Int("code", appErr.ErrorCode),
            logger.String("status", appErr.Status))
        return
    }

    appCtx.Log().Info("API call success",
        logger.String("user_name", apiResp.Name))
}
```

---

## License

MIT