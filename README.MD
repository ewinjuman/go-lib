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
| [`http`](#http-client) | Fluent HTTP client with retry and circuit breaker |
| [`grpc`](#grpc-client) | gRPC client with automatic context and metadata |
| [`error`](#error-handling) | Standardized error types with HTTP/gRPC code mapping |
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
    Filename:    "logs/app.log",
    MaxSize:     100,            // MB before rotation
    MaxBackups:  7,
    MaxAge:      24,             // hours
    Level:       logger.InfoLevel,
}

log, err := logger.New(opts)
if err != nil {
    panic(err)
}
defer log.Shutdown()
```

Or use the global singleton:

```go
logger.InitLogger(opts)
log := logger.GetLogger()
```

### Basic Logging

```go
ctx := context.Background()

log.Info(ctx, "user created", logger.String("user_id", "u-123"))
log.Warn(ctx, "rate limit approaching", logger.Int("remaining", 5))
log.Error(ctx, "payment failed", logger.Error(err), logger.String("order_id", "o-456"))
log.Debug(ctx, "cache miss", logger.String("key", "product:42"))
```

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

Masking example: `"email": "u***@***.com"`, `"token": "ab****xyz"`

### HTTP Request/Response Logging

```go
log.LogRequest(ctx, "POST", "/api/orders", headers, requestBody)
log.LogResponse(ctx, "POST", "/api/orders", 201, responseBody, elapsed)
```

### GORM Database Logging

```go
import gormlogger "gorm.io/gorm/logger"

db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    Logger: log.GormLogger(gormlogger.Info),
})
```

### Options Reference

| Field | Type | Default | Description |
|---|---|---|---|
| `AppName` | string | `""` | Service identifier added to every log |
| `Environment` | string | `""` | Environment tag (dev/staging/prod) |
| `Stdout` | bool | `false` | Print to console |
| `Write` | bool | `false` | Write to file |
| `Filename` | string | `""` | Log file path |
| `MaxSize` | int | `0` | Max file size in MB before rotation |
| `MaxBackups` | int | `0` | Number of rotated files to keep |
| `MaxAge` | int | `0` | Max age in hours for rotated files |
| `Compress` | bool | `false` | Compress rotated log files |
| `Level` | Level | `InfoLevel` | Minimum log level |
| `MaskingPaths` | []string | `nil` | Fields to partially mask |
| `RedactionPaths` | []string | `nil` | Fields to fully redact |
| `DefaultFields` | map[string]string | `nil` | Fields added to every log entry |
| `EnableTrace` | bool | `false` | Attach stack traces to error logs |
| `Development` | bool | `false` | Pretty console output |
| `BufferSize` | int | `1000` | Async log buffer size |
| `FlushInterval` | Duration | `5s` | Auto-flush interval |
| `WorkerPoolSize` | int | `2` | Async worker count |
| `DisableAsync` | bool | `false` | Disable async buffering (synchronous mode) |
| `DisableMasking` | bool | `false` | Disable masking globally |

---

## AppContext

Request-scoped context for propagating request metadata (trace IDs, user IDs, IP, etc.) across service layers.

### Creating a Context

```go
import (
    "github.com/ewinjuman/go-lib/v2/appContext"
    "github.com/ewinjuman/go-lib/v2/logger"
)

log := logger.GetLogger()

appCtx := appContext.New(log).
    SetTraceID("trace-abc-123").
    SetUserID("user-456").
    SetIP("192.168.1.1").
    SetMethod("POST")
```

### From Fiber Request

```go
func handler(c *fiber.Ctx) error {
    appCtx := appContext.FromFiber(c)
    appCtx.LogInfo("handling request")
    return nil
}
```

### Logging via AppContext

```go
appCtx.LogInfo("order placed", logger.String("order_id", "o-789"))
appCtx.LogError("failed to charge card", logger.Error(err))
appCtx.LogWarn("retry attempt", logger.Int("attempt", 2))
appCtx.LogDebug("cache lookup", logger.String("key", "user:123"))
```

### Custom Key-Value Storage

```go
appCtx.Put("tenant_id", "tenant-001")
tenantID := appCtx.Get("tenant_id", "default")

appCtx.Remove("tenant_id")
```

### Propagating to stdlib context

```go
ctx := appCtx.ToContext()

// All fields (trace_id, request_id, user_id, etc.) are accessible
// via the standard context key constants
```

---

## HTTP Client

Fluent HTTP client with automatic logging, retry, circuit breaker, and multipart support.

### Basic Usage

```go
import "github.com/ewinjuman/go-lib/v2/http"

// GET
var result MyResponse
err := http.Get("https://api.example.com", "/users").
    WithQueryParam(map[string]string{"page": "1", "limit": "20"}).
    Execute().
    Consume(&result)

// POST with JSON body
err := http.Post("https://api.example.com", "/users").
    WithBody(CreateUserRequest{Name: "Alice", Email: "alice@example.com"}).
    WithHeaders(map[string]string{"X-API-Key": "my-api-key"}).
    Execute().
    Consume(&result)

// PUT
err := http.Put("https://api.example.com", "/users/{id}").
    WithPathParam(map[string]string{"id": "u-123"}).
    WithBody(updatePayload).
    Execute().
    Consume(&result)

// DELETE
err := http.Delete("https://api.example.com", "/users/{id}").
    WithPathParam(map[string]string{"id": "u-123"}).
    Execute().
    Consume(&result)
```

### Authentication

```go
// Basic auth
http.Get("https://api.example.com", "/protected").
    WithBasicAuth("username", "password").
    Execute()

// Bearer token
http.Get("https://api.example.com", "/protected").
    WithBearer("my-token").
    Execute()
```

### With Request Context & Timeout

```go
http.Post("https://api.example.com", "/orders").
    WithContext(appCtx.ToContext()).
    WithTimeout(30 * time.Second).
    SetRequestID("req-xyz").
    SetDebug(true).
    WithBody(orderPayload).
    Execute().
    Consume(&result)
```

### Multipart File Upload

```go
http.Post("https://api.example.com", "/upload").
    WithFile([]http.MultipartData{
        {Field: "file", Name: "photo.jpg", Path: "/tmp/photo.jpg"},
    }).
    Execute()
```

### Using Full URL

```go
http.GetWithURL("https://api.example.com/v1/users?active=true").Execute()
http.PostWithURL("https://api.example.com/v1/users").WithBody(payload).Execute()
```

### Retry

```go
err := http.Post("https://api.example.com", "/notify").
    WithBody(payload).
    ExecuteWithRetry().
    Consume(&result)
```

### Checking Response

```go
resp := http.Get("https://api.example.com", "/status").Execute()

if resp.IsSuccess() {
    fmt.Println("HTTP", resp.HttpCode())
    resp.Consume(&myStruct)
}

ok, err := resp.IsError()
if ok {
    // handle error
}
```

---

## gRPC Client

Lightweight gRPC client wrapper with context propagation and automatic logging.

### Creating a Connection

```go
import "github.com/ewinjuman/go-lib/v2/grpc"

conn, err := grpc.New(grpc.Options{
    Address: "user-service:5000",
    Timeout: 30 * time.Second,
})
if err != nil {
    log.Fatal(err)
}
```

### Creating a Context for RPC Calls

```go
// CreateContext adds timeout and trace/request IDs to outgoing metadata
ctx, cancel := conn.CreateContext(context.Background(), appCtx)
defer cancel()

// Use ctx with generated gRPC stubs
resp, err := userClient.GetUser(ctx, &pb.GetUserRequest{Id: "u-123"})
```

---

## Error Handling

Standardized error types that map cleanly between HTTP status codes and gRPC codes.

### Creating Errors

```go
import liberror "github.com/ewinjuman/go-lib/v2/error"

// With HTTP status code
err := liberror.New(400, "VALIDATION_ERROR", "invalid email format")

// With optional message (uses HTTP status text if message is omitted)
err := liberror.NewError(404, "NOT_FOUND")
err := liberror.NewError(422, "UNPROCESSABLE", "email already exists")

// Timeout error
err := liberror.DeadlineExceededError("upstream API timed out")
```

### Inspecting Errors

```go
// Get HTTP status code from error (defaults to 500)
code := liberror.GetCode(err)

// Check if timeout
if liberror.IsTimeout(err) {
    // implement retry or fallback
}

// Parse to ApplicationError (works on both gRPC and HTTP errors)
appErr := liberror.ParseError(err)
fmt.Println(appErr.ErrorCode, appErr.Status, appErr.Message)

// Get human-readable status text
text := liberror.StatusMessage(422) // "Unprocessable Entity"
```

### ApplicationError Fields

```go
type ApplicationError struct {
    ErrorCode int    // HTTP status code
    Status    string // e.g. "NOT_FOUND"
    Message   string // human-readable message
}
```

---

## Password

Bcrypt-based password hashing utilities.

```go
import "github.com/ewinjuman/go-lib/v2/password"

// Hash a password before storing
hash, err := password.GeneratePassword("user_secret_password")
if err != nil {
    // handle error
}
// store `hash` in database

// Verify at login
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

// Available keys:
constant.AppContextKey
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

// Retry up to 3 times with incremental backoff
err := retry.Do(ctx, func() error {
    return callExternalAPI()
})

// Retry with custom config
err := retry.DoWithCustom(ctx, myFunc, retry.Config{
    MaxAttempts: 5,
    Delay:       200 * time.Millisecond,
})

// Retry with typed response
result, err := retry.DoWithResponse(ctx, func() (*MyResult, error) {
    return fetchData()
})
```

`ShouldRetry` automatically handles network errors, timeouts, and HTTP 429/502/503.

### Convert

```go
import "github.com/ewinjuman/go-lib/v2/utils/convert"

// Struct-to-struct via JSON
var dst TargetType
err := convert.ObjectToObject(src, &dst)

// Struct to JSON string
jsonStr, err := convert.ObjectToString(myStruct)

// JSON string to struct
err := convert.StringToObject(jsonStr, &myStruct)

// Number to words (Indonesian)
words := convert.ConvertToWord(1500) // "seribu lima ratus"

// Number to words (English)
words := convert.ConvertEng(1500)    // "one thousand five hundred"

// Date to Indonesian formatted string
str := convert.ConvertDateID(time.Now()) // "30 April 2026"
```

### Code Generator

```go
import "github.com/ewinjuman/go-lib/v2/utils/codeGenerator"

// Cryptographically secure random string
code := codeGenerator.RandomString(16, codeGenerator.AlphanumericCharset)
otp  := codeGenerator.RandomString(6, codeGenerator.NumbersCharset)
slug := codeGenerator.RandomString(8, codeGenerator.LowerCaseLettersCharset)
```

Available charsets: `LowerCaseLettersCharset`, `UpperCaseLettersCharset`, `NumbersCharset`, `AlphanumericCharset`, `SpecialCharset`, `AllCharset`.

### General Utilities

```go
import "github.com/ewinjuman/go-lib/v2/utils"

// Check if value exists in slice
found := utils.ContainsInArr([]string{"a", "b", "c"}, "b") // true

// Case-insensitive check
found := utils.ContainsInArrNoCaseSensitive(slice, "Admin")

// String parsing
mid  := utils.BetweenString("hello world foo", "hello ", " foo") // "world"
pre  := utils.BeforeString("user@example.com", "@")              // "user"
post := utils.AfterString("user@example.com", "@")               // "example.com"

// Coalesce: returns first non-empty value
val := utils.Or("", "", "fallback") // "fallback"

// Panic on error (for init-time operations)
cfg := utils.Must(loadConfig())

// Safe pointer dereference
val := utils.SafeDeref(maybeNilPtr) // returns zero value if nil
```

---

## Full Example

```go
package main

import (
    "context"

    appctx  "github.com/ewinjuman/go-lib/v2/appContext"
    liberr  "github.com/ewinjuman/go-lib/v2/error"
    libhttp "github.com/ewinjuman/go-lib/v2/http"
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

    // 2. Create request context
    appCtx := appctx.New(log).
        SetTraceID("trace-abc").
        SetUserID("user-123")

    appCtx.LogInfo("starting request")

    // 3. Hash password
    hash, _ := password.GeneratePassword("secret")
    if !password.ComparePasswords(hash, "secret") {
        appCtx.LogError("password mismatch")
        return
    }

    // 4. Call external API
    type APIResponse struct {
        ID   string `json:"id"`
        Name string `json:"name"`
    }

    var apiResp APIResponse
    err := libhttp.Get("https://api.example.com", "/users/{id}").
        WithContext(appCtx.ToContext()).
        WithPathParam(map[string]string{"id": "u-123"}).
        WithBearer("my-token").
        Execute().
        Consume(&apiResp)

    if err != nil {
        appErr := liberr.ParseError(err)
        appCtx.LogError("API call failed",
            logger.Int("code", appErr.ErrorCode),
            logger.String("status", appErr.Status))
        return
    }

    appCtx.LogInfo("API call success",
        logger.String("user_name", apiResp.Name))

    _ = context.Background()
}
```

---

## License

MIT
