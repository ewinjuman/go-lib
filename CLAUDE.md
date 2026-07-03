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
go test ./cache/...
go test ./database/...

# Run a single test by name
go test -run TestRequest_DoRequest ./httpclient/...

# Run benchmarks
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
| `httpclient` | Fluent HTTP client (stdlib only — no external HTTP library dependencies) |
| `apperror` | `ApplicationError` type bridging HTTP status codes and gRPC codes |
| `cache` | Redis client (standalone/Sentinel/Cluster via `redis.UniversalClient`), typed `Store[T]`, stampede-protected `ObjectCache[T]`, distributed `Lock` |
| `database` | GORM connection factory for MySQL/PostgreSQL, wired with `logger` |
| `grpc` | gRPC client wrapper with context/metadata propagation |
| `constant` | Context key constants shared across packages |
| `password` | bcrypt hashing helpers |
| `utils` | String helpers, retry, struct conversion, code generation |
| `bench` | Benchmark tests for `httpclient` |
| `examples` | Runnable usage examples (not production code) |

### AppContext

`appContext.New(ctx context.Context, log *Logger)` — `ctx` is stored as `parentCtx` so `ToContext()` chains from it rather than `context.Background()`, preserving deadlines and cancellation. `log` is optional; `nil` falls back to `logger.GetLogger()`.

**All fields are unexported.** Use the `Set*` / `Get*` accessors:
- Propagated to context via `ToContext()`: `requestID`, `traceID`, `userID`, `tenantID`, `requestTime`, `method`, `url`, `ip`, `userAgent`
- Local-only (never added to context): `responseStatus`, `port`, `srcIP`, `header`, `request`

**`FromFiber(c *fiber.Ctx)`** never returns nil — if no AppContext is found in `c.Locals`, it returns `New(c.UserContext(), nil)` so callers never need a nil-check.

**`FromContext(ctx context.Context) *AppContext`** retrieves the AppContext from any stdlib context. Returns nil if not found. Works because `buildContext()` always stores the AppContext itself under `constant.AppContextKey` in the context chain. This allows service layers that only receive `ctx` to access the full AppContext without Fiber dependency.

**`Clone() *AppContext`** creates a derived AppContext with a new `requestID` and `requestTime`, copying all propagated identity fields (`traceID`, `userID`, `tenantID`, `ip`, `userAgent`, `url`, `method`). Local-only fields and `cMap` are not copied. Used for outgoing service-to-service calls so each hop has its own requestID while sharing the same traceID.

**`Duration() time.Duration`** returns `time.Since(requestTime)` under RLock. Used in after-middleware for response time logging.

**`SetResponseStatus(int)` / `GetResponseStatus() int`** stores the HTTP response status code locally (never propagated to context since it's known only after the response is sent). Used in after-middleware.

**`SetTenantID(string)` / `GetTenantID() string`** propagated to context under `constant.TenantIDKey`. Added for multi-tenant SaaS patterns.

**`SetURL(url string) *AppContext`** propagates to context under `constant.RequestPathKey`.

**`ToContext()` caches its result.** On the first call (or after any propagated-field `Set*`), it builds the context chain (up to 10 `context.WithValue` calls, including AppContext itself) under a write lock, stores it in `cachedCtx`, and returns it. Subsequent calls return the cached pointer under an RLock. The cache is invalidated (`cachedCtx = nil`) inside every `Set*` method that touches a propagated field. Local-only setters (`SetResponseStatus`, `SetPort`, `SetSrcIP`, `SetHeader`, `SetRequest`) do **not** invalidate the cache.

**All field access is protected by `sync.RWMutex`** — all getters use `RLock`, all setters use `Lock`.

**Key-value store** (`Put`/`Get`/`Remove`) uses stdlib `sync.Map` — no external dependency. The `github.com/orcaman/concurrent-map` dependency was removed.

**`grpc/client.go`** uses `appCtx.GetRequestID()` (not the old public field `appCtx.RequestID`).

`Log()` returns a `*logger.ContextualLogger` already bound to the current request context. Callers never pass `ToContext()` manually:

```go
appCtx.Log().Info("msg", logger.String("k", "v"))
appCtx.Log().Error("msg", logger.Error(err))
```

`ContextualLogger` (`logger/contextual.go`) wraps `*Logger` + `context.Context` and proxies all log methods (Debug/Info/Warn/Error/Fatal) and utility methods (LogRequestHttp, LogResponseHttp, LogRequestGrpc, LogResponseGrpc, etc.) without requiring a ctx argument. Use `.Underlying()` to access the raw `*Logger` if needed.

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

### Logger

Async by default — buffered channel + `WorkerPoolSize` goroutines (default 2). Call `log.Shutdown()` to flush before process exit — **idempotent** (safe to call multiple times; uses `sync.Once` internally). `log.Flush()` signals all workers via a buffered `flushCh` (capacity = `WorkerPoolSize`) so signals are not lost while workers are busy. Caller is captured at the call site before async dispatch (in `LogEntry.Caller`) using `utils.FileWithLineNum()` to survive the goroutine hop.

**File rotation** is handled by `dailyRotatingWriter` (`logger/rotating_writer.go`), which wraps `lumberjack.Logger`. The active log file is named with the current date: `<basename>-YYYY-MM-DD<ext>` (e.g. `logs/app-2026-05-25.log`). When midnight passes, the next `Write` call transparently opens a new dated file — no process restart needed. Size-based rotation within a day is still managed by lumberjack via `MaxSize`. `MaxAge` (in **days**) and `MaxBackups` apply to rotated backup files only.

**Async log timing:** `LogEntry.Timestamp` is captured at the call site and written as `logged_at` in each entry, showing when the event actually occurred. The `timestamp` field is set by zap when the worker processes the entry. Typical delay between the two is < `FlushInterval` (500ms default).

**`GetLogger()`** performs the `instance == nil` check inside `once.Do` — thread-safe, no data race on concurrent first calls.

**`GormLogger.Info/Warn/Error`** delegate directly to `l.Logger.Info/Warn/Error` (no extra `go` wrapper) since the logger is already async. `GormLogger.Trace` keeps its own goroutine because it runs non-trivial computation (`fc()`) before logging.

`MaskingPaths` → partial masking (e.g. email → `u***@***.com`, only TLD shown in domain). `RedactionPaths` → `[REDACTED]`. Both are case-insensitive and cached in a `sync.Map`. `maskMap` unwraps `reflect.Interface` before checking kind — correctly masks string values inside `map[string]interface{}`. GORM integration is in `logger/gorm_logger.go`.

**`*Logger.WithContext(ctx) *ContextualLogger`** — bind a context once and log without passing `ctx` on every call. Returns a `*ContextualLogger` whose `Debug/Info/Warn/Error/Fatal` methods need no ctx argument. Equivalent to `appCtx.Log()` but works anywhere you have a `*Logger` and a `context.Context`. Internally, the previously-exported `WithContext` that returned `*zap.Logger` has been renamed `zapWithContext` (unexported) so the name is free for this public API.

### Error Package (`apperror`)

`ApplicationError` carries `ErrorCode` (HTTP int), `Status` (string), `Message` (string), and an unexported `cause error`. Named `apperror` (not `error`) to avoid shadowing the Go builtin.

**Two files:**
- `apperror/error.go` — core type, `New`, `NewError`, `ParseError`, `GetCode`, `IsTimeout`, `StatusMessage`, `DeadlineExceededError`, gRPC↔HTTP maps
- `apperror/constructors.go` — named constructors, sentinel variables, named predicates

**Named constructors** return `*ApplicationError` (not `error`) so callers can chain `.WithCause` without a type assertion: `apperror.NotFound("x").WithCause(sql.ErrNoRows)`.

**Sentinel variables** (`ErrNotFound`, `ErrBadRequest`, etc.) allow `errors.Is` checks across error-wrapping boundaries. Two `ApplicationError`s are equal when `ErrorCode` **and** `Status` match — `Message` is ignored so `errors.Is(NotFound("x"), ErrNotFound)` returns `true`.

**`WithCause(err error) *ApplicationError`** returns a **new** instance with the same ErrorCode/Status/Message and `cause` set. It does **not** mutate the original — sentinel errors can be shared safely.

**`Unwrap() error`** exposes `cause` so `errors.Is`/`errors.As` traverse the full chain: `errors.Is(NotFound("x").WithCause(io.EOF), io.EOF)` returns `true`.

**`ToGRPCStatus() *status.Status`** converts an `ApplicationError` to a gRPC status using the reverse HTTP→gRPC map (`httpCodeToRPCCode`). Unmapped codes fall back to `codes.Unknown`.

**`ParseError(err)`** checks in order: (1) nil → nil, (2) gRPC status error → `codeApplication` map, (3) `*ApplicationError` via `errors.As` (handles wrapped errors), (4) plain error wrapped in 500. String-splitting on `=` extracts the last segment of context-style errors.

**Named predicates** (`IsNotFound`, `IsBadRequest`, etc.) are shorthand for `errors.Is(err, ErrXxx)` — they work through any wrapping layer.

**`rpcCodeToApplicationCode`** maps gRPC → HTTP. **`httpCodeToRPCCode`** maps HTTP → gRPC (reverse direction for `ToGRPCStatus`). Both are package-level maps; `codeApplication` and `httpCodeToGRPCCode` are the lookup helpers.

### Cache Package (`cache`)

`RedisClient` wraps `redis.UniversalClient` (not the concrete `*redis.Client`), so the same wrapper works over a standalone connection, a Sentinel-backed failover client, or a `*redis.ClusterClient`.

**`NewRedisClient(RedisOption)`** dials via `redis.NewUniversalClient`, which selects the concrete client type from the option shape:
- `len(Addresses) > 1` or `IsClusterMode: true` → `*redis.ClusterClient`
- `MasterName` set → Sentinel failover (go-redis implements this as a specially-dialed `*redis.Client`, not a distinct type — do not type-assert to detect Sentinel mode)
- otherwise → standalone `*redis.Client`

`resolveAddrs(config)` picks `Addresses` when non-empty, else falls back to the single `Address` field — kept as its own function so address-selection logic is unit-testable without a reachable Redis.

**`WrapRedisClient(client redis.UniversalClient) *RedisClient`** wraps an already-connected client instead of dialing a new one — use when the caller already owns a shared client wired elsewhere.

**`pingRedis`** calls `client.Ping(ctx)` directly (not `.Conn().Ping(ctx)`) so it works identically across standalone, Sentinel, and cluster clients — `Conn()` only exists on the concrete `*redis.Client` type.

**`Store[T]`** (`store.go`) is a generic single-type Redis cache with a fixed TTL; JSON (de)serialization goes through `utils/convert`.

**`ObjectCache[T]`** (`object_cache.go`) adds stampede protection on top of `Store[T]`: an in-process `singleflight.Group` coalesces concurrent goroutines within a pod, and a Redis `Lock` (SETNX-based, `lock.go`) coalesces across pods. The lock winner double-checks the cache, loads, and populates; everyone else polls via `waitForCache` for up to `LockWait` before falling back to a direct (uncached) load. `Policy` (`policy.go`) controls whether a given key is cacheable at all — `PolicyAll`, `PolicyNone`, `PolicyKeys`, `PolicyFunc`.

**`Lock`** (`lock.go`) is acquired via `RedisClient.TryLock` (SETNX with a random UUID token) and released via a Lua script that only deletes the key if it still holds the caller's token — an expired-and-reacquired lock is never released out from under its new holder.

**`cache` has zero dependency on `logger` or `apperror`** — every method returns a plain `error`. Callers are free to wrap results however they want.

### Database Package (`database`)

**`NewConnection(Option, *logger.Logger)`** dispatches to `createMysqlConnection` or `createPostgresConnection` based on `Option.DbType` (`""` defaults to postgres), pings the connection, then wires GORM's query logger via `Logger.NewGormLogger`.

**The `*logger.Logger` parameter is a hard dependency on go-lib's concrete logger type** (not an interface) — this is the one place in `cache`/`database` that isn't logger-agnostic, because GORM's logging hook is wired at connection time. Passing `nil` is only safe when `Option.LogMode` is `false` (`gormlogger.Silent`); with `LogMode: true` and a nil logger, `GormLogger.Info/Warn/Error` dereference it directly and panic.

### Key Design Rules

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
- **`FromContext` may return nil** — always nil-check the result; it returns nil when the context was not built from an AppContext.
- **`ToContext()` cache invalidation is automatic** — call `Set*` before any `Log()` call during request setup; the first `Log()` builds and caches the context. Do not manually call `ToContext()` in hot paths just to pre-warm it.
- **Local-only AppContext fields** (`responseStatus`, `port`, `srcIP`, `header`, `request`) are never added to the context chain — do not add them to `buildContext()`.
- **`Clone()` for outgoing calls** — always use `Clone()` instead of passing the original AppContext to `grpc.CreateContext` or similar, so each hop gets a unique `requestID` while sharing `traceID`.
- **`constant.TenantIDKey`** is now defined — use it to read tenant ID from stdlib context: `ctx.Value(constant.TenantIDKey).(string)`.
- **Named constructors are preferred over `New`/`NewError`** for common HTTP codes — use `apperror.NotFound(...)`, `apperror.BadRequest(...)`, etc. Use `New`/`NewError` only for non-standard status strings or unusual codes.
- **`WithCause` returns a new instance** — it never mutates the original. Safe to call on package-level sentinel variables.
- **`Is()` matches by `ErrorCode` + `Status` only** — `Message` is intentionally excluded so `errors.Is(NotFound("custom"), ErrNotFound)` returns `true`.
- **`ToGRPCStatus()` is for gRPC handler returns** — call `.Err()` on the result to get the gRPC-compatible error: `return nil, ae.ToGRPCStatus().Err()`.
- **Prefer `log.WithContext(ctx)` over passing `ctx` per call** — when multiple log statements share the same context, bind once: `clog := log.WithContext(ctx)` then use `clog.Info/Error/...`. Do not call `log.Info(ctx, ...)` in a loop with the same ctx.
- **`zapWithContext` is unexported** — do not rename it back or expose it; its purpose is internal async-worker enrichment only. The public `WithContext` on `*Logger` returns `*ContextualLogger`, not `*zap.Logger`.
- **`cache.RedisClient` holds `redis.UniversalClient`, never `*redis.Client`** — do not narrow the field or a method signature back to the concrete type, or Sentinel/Cluster support silently breaks.
- **Sentinel failover is not a distinct Go type** — `redis.NewFailoverClient` returns `*redis.Client`; never write a type assertion expecting a `*redis.FailoverClient` to detect Sentinel mode.
- **`database.NewConnection`'s `logger` param may only be `nil` when `LogMode: false`** — with `LogMode: true`, a nil logger panics inside `GormLogger.Info/Warn/Error`.
- **`cache` and `database` never import `apperror`** — they return plain `error`; do not add an `apperror` dependency here, callers decide their own error-wrapping convention.
