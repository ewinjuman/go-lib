# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build all packages
go build ./...

# Run all tests
go test ./...

# Run tests for a specific package
go test ./httpclient/...
go test ./logger/...
go test ./apperror/...

# Run a single test by name
go test -run TestRequest_DoRequest ./httpclient/...

# Run benchmarks (httpclient vs httpstd comparison)
go test -run=^$ -bench=. -benchmem -benchtime=3s ./bench/...
```

## Architecture

### Module
`github.com/ewinjuman/go-lib/v2` — personal utility library for building Go microservices.

### Package Map

| Package | Role |
|---------|------|
| `logger` | Structured async logger (zap-backed) with masking, redaction, file rotation, GORM integration |
| `appContext` | Request-scoped context carrier — propagates trace ID, user ID, IP, method across service layers |
| `httpclient` | Fluent HTTP client using [resty](https://github.com/go-resty/resty) |
| `httpstd` | Clone of `httpclient` using stdlib `net/http` — created for performance benchmarking |
| `apperror` | `ApplicationError` type bridging HTTP status codes and gRPC codes |
| `grpc` | gRPC client wrapper with context/metadata propagation |
| `constant` | Context key constants shared across packages |
| `password` | bcrypt hashing helpers |
| `utils` | String helpers, retry, struct conversion, code generation |
| `bench` | Benchmark tests comparing `httpclient` (resty) vs `httpstd` (net/http) |
| `examples` | Runnable usage examples (not production code) |

### AppContext

`appContext.New(ctx context.Context, log *Logger)` — `ctx` is stored as `parentCtx` so `ToContext()` chains from it rather than `context.Background()`, preserving deadlines and cancellation. `log` is optional; `nil` falls back to `logger.GetLogger()`.

`Log()` returns a `*logger.ContextualLogger` already bound to the current request context. Callers never pass `ToContext()` manually:

```go
appCtx.Log().Info("msg", logger.String("k", "v"))
appCtx.Log().Error("msg", logger.Error(err))
```

`ContextualLogger` (`logger/contextual.go`) wraps `*Logger` + `context.Context` and proxies all log methods (Debug/Info/Warn/Error/Fatal) and utility methods (LogRequestHttp, LogResponseHttp, LogRequestGrpc, LogResponseGrpc, etc.) without requiring a ctx argument. Use `.Underlying()` to access the raw `*Logger` if needed.

### HTTP Client Design (`httpclient` and `httpstd`)

Both packages expose an identical fluent API:

```go
httpclient.Post("https://api.example.com/users").
    WithBody(payload).
    WithRequestID("abc").
    WithTimeout(5 * time.Second).
    Execute().
    Consume(&result)
```

**Builder flow:** `Post(url)` → `*RequestBuilder` → chain `With*` → `Execute()` → `*Response` → `Consume(&v)` / `IsSuccess()` / `IsError()`.

**Circuit breaker** is on by default, global per-host (keyed by `scheme://host` in a `sync.Map`). Config is applied only on first request to a host — subsequent requests reuse the same CB. Use `.WithoutCircuitBreaker()` or `.WithCircuitBreakerConfig(cfg)` as needed.

**Success codes**: defaults to `[200]`. `Response.SuccessCodes` is set at the start of `doRequest` so `IsSuccess()`, `IsError()`, and `Consume()` all use the same source — no divergence.

**`httpclient` vs `httpstd`**: `httpstd` uses a `sync.Pool` for `bytes.Buffer` in JSON encoding and is generally faster on parallel benchmarks. Any change to the public API, circuit breaker logic, or response handling **must be mirrored in both packages**.

### Logger

Async by default — buffered channel + `WorkerPoolSize` goroutines (default 2). Call `log.Shutdown()` to flush before process exit; `log.Flush()` notifies all workers. Caller is captured at the call site before async dispatch (in `LogEntry.Caller`) using `utils.FileWithLineNum()` to survive the goroutine hop.

`MaskingPaths` → partial masking (e.g. email → `u***@***.com`). `RedactionPaths` → `[REDACTED]`. Both are case-insensitive and cached in a `sync.Map`. GORM integration is in `logger/gorm_logger.go`.

### Error Package (`apperror`)

`ApplicationError` carries `ErrorCode` (HTTP int), `Status` (string), `Message` (string). `ParseError(err)` handles gRPC `status.Error`, `*ApplicationError`, and plain errors uniformly. Named `apperror` (not `error`) to avoid shadowing the Go builtin.

### Key Design Rules

- **`httpclient` and `httpstd` are kept in sync** — mirror all API and logic changes in both packages.
- **Circuit breaker state is global per host**, not per `RequestBuilder`. A new builder per request is fine.
- **`AppContext.New()` takes `ctx` first** — always pass the request context (e.g. `c.UserContext()` in Fiber) so deadlines propagate into `ToContext()`.
- **`Log()` returns `*ContextualLogger`**, not `*Logger` — do not bypass it by calling `ac.Log().Underlying().Info(ctx, ...)` unless raw zap access is truly needed.
- **README.MD is the source of truth for public API examples** — keep it in sync after any API change.