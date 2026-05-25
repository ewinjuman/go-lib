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
go test ./appContext/...

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
| `httpstd` | Clone of `httpclient` using stdlib `net/http` — identical public API, generally faster under high concurrency |
| `apperror` | `ApplicationError` type bridging HTTP status codes and gRPC codes |
| `grpc` | gRPC client wrapper with context/metadata propagation |
| `constant` | Context key constants shared across packages |
| `password` | bcrypt hashing helpers |
| `utils` | String helpers, retry, struct conversion, code generation |
| `bench` | Benchmark tests comparing `httpclient` (resty) vs `httpstd` (net/http) |
| `examples` | Runnable usage examples (not production code) |

### AppContext

`appContext.New(ctx context.Context, log *Logger)` — `ctx` is stored as `parentCtx` so `ToContext()` chains from it rather than `context.Background()`, preserving deadlines and cancellation. `log` is optional; `nil` falls back to `logger.GetLogger()`.

**All fields are unexported.** Use the `Set*` / `Get*` accessors:
- Propagated to context via `ToContext()`: `requestID`, `traceID`, `userID`, `requestTime`, `method`, `url`, `ip`, `userAgent`
- Local-only (never added to context): `port`, `srcIP`, `header`, `request`

**`FromFiber(c *fiber.Ctx)`** never returns nil — if no AppContext is found in `c.Locals`, it returns `New(c.UserContext(), nil)` so callers never need a nil-check.

**`SetURL(url string) *AppContext`** was added alongside the other setters; `url` maps to `constant.RequestPathKey` in the context.

**`ToContext()` caches its result.** On the first call (or after any propagated-field `Set*`), it builds the context chain (up to 8 `context.WithValue` calls) under a write lock, stores it in `cachedCtx`, and returns it. Subsequent calls return the cached pointer under an RLock. The cache is invalidated (`cachedCtx = nil`) inside every `Set*` method that touches a propagated field. Local-only setters (`SetPort`, `SetSrcIP`, `SetHeader`, `SetRequest`) do **not** invalidate the cache.

**All field access is protected by `sync.RWMutex`** — all getters use `RLock`, all setters use `Lock`.

**Key-value store** (`Put`/`Get`/`Remove`) uses stdlib `sync.Map` — no external dependency. The `github.com/orcaman/concurrent-map` dependency was removed.

**`grpc/client.go`** uses `appCtx.GetRequestID()` (not the old public field `appCtx.RequestID`).

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

**Builder flow:** `Post(url)` → `*RequestBuilder` → chain `With*` → `Execute()` → `*Response` → `Consume(&v)` / `IsSuccess()` / `IsError()` / `SaveToFile(path)`.

**File download:** `SaveToFile(path)` writes the buffered `Response.Body` to disk (small files). `WithOutput(w io.Writer)` streams directly to any writer without buffering — `Response.Body` is `nil` after streaming; never call `Consume` or `SaveToFile` on the same response. `httpclient` uses `SetDoNotParseResponse(true)` + `RawResponse.Body` for true streaming; `httpstd` uses `io.Copy` directly from `resp.Body`.

**Circuit breaker** is on by default, global per-host (keyed by `scheme://host` in a `sync.Map`). Config is applied only on first request to a host — subsequent requests reuse the same CB. Use `.WithoutCircuitBreaker()` or `.WithCircuitBreakerConfig(cfg)` as needed.

**Success codes**: defaults to `[200]`. `Response.SuccessCodes` is set at the start of `doRequest` so `IsSuccess()`, `IsError()`, and `Consume()` all use the same source — no divergence.

**`httpclient` vs `httpstd`**: `httpstd` uses a `sync.Pool` for `bytes.Buffer` in JSON encoding and is generally faster on parallel benchmarks. Any change to the public API, circuit breaker logic, or response handling **must be mirrored in both packages**.

### Logger

Async by default — buffered channel + `WorkerPoolSize` goroutines (default 2). Call `log.Shutdown()` to flush before process exit — **idempotent** (safe to call multiple times; uses `sync.Once` internally). `log.Flush()` signals all workers via a buffered `flushCh` (capacity = `WorkerPoolSize`) so signals are not lost while workers are busy. Caller is captured at the call site before async dispatch (in `LogEntry.Caller`) using `utils.FileWithLineNum()` to survive the goroutine hop.

**File rotation** is handled by `dailyRotatingWriter` (`logger/rotating_writer.go`), which wraps `lumberjack.Logger`. The active log file is named with the current date: `<basename>-YYYY-MM-DD<ext>` (e.g. `logs/app-2026-05-25.log`). When midnight passes, the next `Write` call transparently opens a new dated file — no process restart needed. Size-based rotation within a day is still managed by lumberjack via `MaxSize`. `MaxAge` (in **days**) and `MaxBackups` apply to rotated backup files only.

**Async log timing:** `LogEntry.Timestamp` is captured at the call site and written as `logged_at` in each entry, showing when the event actually occurred. The `timestamp` field is set by zap when the worker processes the entry. Typical delay between the two is < `FlushInterval` (500ms default).

**`GetLogger()`** performs the `instance == nil` check inside `once.Do` — thread-safe, no data race on concurrent first calls.

**`GormLogger.Info/Warn/Error`** delegate directly to `l.Logger.Info/Warn/Error` (no extra `go` wrapper) since the logger is already async. `GormLogger.Trace` keeps its own goroutine because it runs non-trivial computation (`fc()`) before logging.

`MaskingPaths` → partial masking (e.g. email → `u***@***.com`, only TLD shown in domain). `RedactionPaths` → `[REDACTED]`. Both are case-insensitive and cached in a `sync.Map`. `maskMap` unwraps `reflect.Interface` before checking kind — correctly masks string values inside `map[string]interface{}`. GORM integration is in `logger/gorm_logger.go`.

### Error Package (`apperror`)

`ApplicationError` carries `ErrorCode` (HTTP int), `Status` (string), `Message` (string). `ParseError(err)` handles gRPC `status.Error`, `*ApplicationError`, and plain errors uniformly. Named `apperror` (not `error`) to avoid shadowing the Go builtin.

### Key Design Rules

- **`httpclient` and `httpstd` are kept in sync** — mirror all API and logic changes in both packages.
- **Circuit breaker state is global per host**, not per `RequestBuilder`. A new builder per request is fine.
- **`AppContext.New()` takes `ctx` first** — always pass the request context (e.g. `c.UserContext()` in Fiber) so deadlines propagate into `ToContext()`.
- **`Log()` returns `*ContextualLogger`**, not `*Logger` — do not bypass it by calling `ac.Log().Underlying().Info(ctx, ...)` unless raw zap access is truly needed.
- **README.MD is the source of truth for public API examples** — keep it in sync after any API change.
- **`WithOutput` and `Consume`/`SaveToFile` are mutually exclusive** — after `WithOutput`, `Response.Body` is nil; calling `Consume` or `SaveToFile` returns `ErrEmptyResponseBody`.
- **`log.Shutdown()` is idempotent** — safe to `defer` in multiple places (main + signal handler). Second call is a no-op.
- **`MaxAge` in logger `Options` is in days**, not hours — matches lumberjack's unit.
- **Log filename includes date** — `opts.Filename` is the base path; `dailyRotatingWriter` appends `-YYYY-MM-DD` before the extension. Never hardcode a dated filename in `opts.Filename`.
- **`AppContext` fields are all unexported** — never add public fields to `AppContext`; always expose via `Set*`/`Get*` methods with mutex protection.
- **`FromFiber` is always safe to call** — it never returns nil; the nil-fallback is `New(c.UserContext(), nil)`. No nil-check needed at call sites.
- **`ToContext()` cache invalidation is automatic** — call `Set*` before any `Log()` call during request setup; the first `Log()` builds and caches the context. Do not manually call `ToContext()` in hot paths just to pre-warm it.
- **Local-only AppContext fields** (`port`, `srcIP`, `header`, `request`) are never added to the context chain — do not add new local-only fields to `buildContext()`.
