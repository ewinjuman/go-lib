// examples/http/main.go
//
// Runnable examples for the unified httpclient package.
// Each example is a self-contained function — comment/uncomment in main() as needed.
//
// Public endpoints used:
//
//	https://httpbin.org  — returns request metadata as JSON, great for testing
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ewinjuman/go-lib/v2/examples/helper"
	"github.com/ewinjuman/go-lib/v2/httpclient"
	"github.com/ewinjuman/go-lib/v2/logger"
)

func main() {
	examplePackageLevelShortcut()
	exampleNamedClientWithMiddleware()
	exampleJSONPost()
	exampleFormPost()
	exampleMultipartUpload()
	exampleGraphQL()
	examplePathAndQueryParams()
	exampleBearerAuth()
	exampleBasicAuth()
	exampleResponseHeaders()
	exampleWithSuccessCodes()
	exampleStreamingDownload()
	exampleContextCancellation()
	exampleRetryMiddleware()
	exampleCircuitBreaker()
	exampleSSE()
}

// ── 1. Package-level shortcut (backward compatible) ───────────────────────────
//
// Post/Get/Put/Delete/Patch/Options are package-level wrappers around a shared
// default client with a 30-second timeout. No middleware by default.
// Use these for simple one-off requests; prefer a named Client for production.
func examplePackageLevelShortcut() {
	// log.WithContext binds ctx once — no need to pass it on every call.
	log := helper.GetLogger().WithContext(context.Background())

	type Result struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}

	var result Result
	err := httpclient.Get("https://httpbin.org/get").
		WithRequestID("req-001").
		WithQueryParams(map[string]string{"page": "1", "limit": "10"}).
		Execute().
		Consume(&result)
	if err != nil {
		log.Error("shortcut GET failed", logger.Error(err))
		return
	}
	log.Info("shortcut GET", logger.String("url", result.URL))
}

// ── 2. Named Client with full middleware stack ────────────────────────────────
//
// In production, create one Client per upstream service and reuse it.
// Middlewares are applied in registration order — first registered = outermost.
// Logging → Retry → CircuitBreaker is the recommended order.
func exampleNamedClientWithMiddleware() {
	log := helper.GetLogger().WithContext(context.Background())

	client := httpclient.New(
		httpclient.WithBaseURL("https://httpbin.org"),
		httpclient.WithDefaultTimeout(10*time.Second),
		httpclient.WithDefaultHeaders(map[string]string{
			"X-App-Name": "go-lib-example",
		}),
		httpclient.WithMiddleware(httpclient.LoggingMiddleware(log.Underlying())),
		httpclient.WithMiddleware(httpclient.RetryMiddleware(httpclient.RetryConfig{
			MaxAttempts: 3,
			Backoff:     httpclient.ExponentialBackoff(200*time.Millisecond, 2.0),
			RetryOn:     httpclient.RetryOnAny,
		})),
		httpclient.WithMiddleware(httpclient.CircuitBreakerMiddleware(
			httpclient.CircuitBreakerConfig{FailureThreshold: 5},
		)),
	)

	type Result struct {
		URL string `json:"url"`
	}
	var result Result

	err := client.Get("/get").
		WithQueryParams(map[string]string{"from": "named-client"}).
		Execute().
		Consume(&result)
	if err != nil {
		log.Error("named client GET failed", logger.Error(err))
		return
	}
	log.Info("named client GET", logger.String("url", result.URL))
}

// ── 3. JSON POST body ─────────────────────────────────────────────────────────
//
// WithBody encodes the value as JSON and sets Content-Type: application/json.
func exampleJSONPost() {
	log := helper.GetLogger().WithContext(context.Background())

	type Payload struct {
		UserID string `json:"user_id"`
		Action string `json:"action"`
	}
	type Result struct {
		JSON Payload `json:"json"`
	}

	var result Result
	err := httpclient.Post("https://httpbin.org/post").
		WithBody(Payload{UserID: "u123", Action: "login"}).
		Execute().
		Consume(&result)
	if err != nil {
		log.Error("JSON POST failed", logger.Error(err))
		return
	}
	log.Info("JSON POST", logger.String("user_id", result.JSON.UserID))
}

// ── 4. Form-encoded POST ──────────────────────────────────────────────────────
//
// WithForm encodes the value as application/x-www-form-urlencoded.
// The value must be convertible to map[string]string.
func exampleFormPost() {
	log := helper.GetLogger().WithContext(context.Background())

	type FormResult struct {
		Form map[string]string `json:"form"`
	}

	var result FormResult
	err := httpclient.Post("https://httpbin.org/post").
		WithForm(map[string]string{"username": "alice", "role": "admin"}).
		Execute().
		Consume(&result)
	if err != nil {
		log.Error("form POST failed", logger.Error(err))
		return
	}
	log.Info("form POST", logger.String("username", result.Form["username"]))
}

// ── 5. Multipart/form-data upload ────────────────────────────────────────────
//
// WithMultipart accepts Field() for text fields, FileFromReader() for in-memory
// files, and FileFromPath() for files read from disk.
func exampleMultipartUpload() {
	log := helper.GetLogger().WithContext(context.Background())

	// Simulate an in-memory file (e.g., a generated CSV).
	csvContent := strings.NewReader("id,name\n1,alice\n2,bob")

	type Result struct {
		Form  map[string]string `json:"form"`
		Files map[string]string `json:"files"`
	}

	var result Result
	err := httpclient.Post("https://httpbin.org/post").
		WithMultipart(
			httpclient.Field("description", "user export"),
			httpclient.Field("format", "csv"),
			httpclient.FileFromReader("file", "users.csv", csvContent),
		).
		Execute().
		Consume(&result)
	if err != nil {
		log.Error("multipart POST failed", logger.Error(err))
		return
	}
	log.Info("multipart POST",
		logger.String("description", result.Form["description"]),
		logger.String("format", result.Form["format"]),
	)
}

// ── 6. GraphQL ────────────────────────────────────────────────────────────────
//
// WithGraphQL is sugar that builds {"query": q, "variables": vars} as JSON.
func exampleGraphQL() {
	log := helper.GetLogger().WithContext(context.Background())

	// httpbin.org/post echoes the JSON body, so we can verify the shape.
	type Body struct {
		JSON map[string]any `json:"json"`
	}

	var result Body
	err := httpclient.Post("https://httpbin.org/post").
		WithGraphQL(`{ user(id: "1") { name email } }`, map[string]any{"id": "1"}).
		Execute().
		Consume(&result)
	if err != nil {
		log.Error("GraphQL POST failed", logger.Error(err))
		return
	}
	log.Info("GraphQL POST", logger.Interface("query", result.JSON["query"]))
}

// ── 7. Path params + query params ────────────────────────────────────────────
//
// WithPathParam replaces :key placeholders in the URL path.
// Each call to WithPathParam replaces the entire map — pass all params in one call.
// WithQueryParams appends ?key=value to the URL.
func examplePathAndQueryParams() {
	log := helper.GetLogger().WithContext(context.Background())

	type Result struct {
		URL string `json:"url"`
	}
	var result Result

	// :resource is replaced → /anything/orders
	err := httpclient.Get("https://httpbin.org/anything/:resource").
		WithPathParam(map[string]string{"resource": "orders"}).
		WithQueryParams(map[string]string{"page": "2", "per_page": "20"}).
		Execute().
		Consume(&result)
	if err != nil {
		log.Error("path param GET failed", logger.Error(err))
		return
	}
	log.Info("path + query params", logger.String("url", result.URL))
}

// ── 8. Bearer token auth ──────────────────────────────────────────────────────
//
// WithBearer sets Authorization: Bearer <token> on this request.
// WithDefaultBearer on the Client accepts a func() string so tokens can rotate
// — the function is called on every Execute().
func exampleBearerAuth() {
	log := helper.GetLogger().WithContext(context.Background())

	type Result struct {
		Headers map[string]string `json:"headers"`
	}
	var result Result

	err := httpclient.Get("https://httpbin.org/bearer").
		WithBearer("my-jwt-token-here").
		Execute().
		Consume(&result)
	if err != nil {
		log.Error("bearer GET failed", logger.Error(err))
		return
	}
	log.Info("bearer auth", logger.String("authorization", result.Headers["Authorization"]))
}

// ── 9. Basic auth ─────────────────────────────────────────────────────────────
//
// WithBasicAuth encodes user:pass in Base64 and sets Authorization: Basic <…>.
// WithDefaultBasicAuth on the Client applies it to every request.
func exampleBasicAuth() {
	log := helper.GetLogger().WithContext(context.Background())

	type Result struct {
		Authenticated bool   `json:"authenticated"`
		User          string `json:"user"`
	}
	var result Result

	err := httpclient.Get("https://httpbin.org/basic-auth/alice/s3cr3t").
		WithBasicAuth("alice", "s3cr3t").
		Execute().
		Consume(&result)
	if err != nil {
		log.Error("basic auth GET failed", logger.Error(err))
		return
	}
	log.Info("basic auth",
		logger.String("user", result.User),
		logger.Bool("authenticated", result.Authenticated),
	)
}

// ── 10. Inspecting response headers ──────────────────────────────────────────
//
// Response.Headers (http.Header) is always populated on non-streaming requests.
// Use resp.Raw() to access the underlying *http.Response for advanced use cases.
func exampleResponseHeaders() {
	log := helper.GetLogger().WithContext(context.Background())

	resp := httpclient.Get("https://httpbin.org/response-headers").
		WithQueryParams(map[string]string{"X-Custom-Header": "hello"}).
		Execute()
	if resp.Error != nil {
		log.Error("response headers failed", logger.Error(resp.Error))
		return
	}
	log.Info("response headers",
		logger.Int("status", resp.HttpCode()),
		logger.String("content-type", resp.Headers.Get("Content-Type")),
		logger.String("x-custom-header", resp.Headers.Get("X-Custom-Header")),
	)
}

// ── 11. Custom success codes ──────────────────────────────────────────────────
//
// By default only 200 is considered success. WithSuccessCodes overrides this.
// IsSuccess(), IsError(), and Consume() all use the same codes — no divergence.
func exampleWithSuccessCodes() {
	log := helper.GetLogger().WithContext(context.Background())

	resp := httpclient.Post("https://httpbin.org/status/201").
		WithBody(map[string]string{"note": "created"}).
		WithSuccessCodes([]int{200, 201, 202}).
		Execute()

	isErr, _ := resp.IsError()
	log.Info("custom success codes",
		logger.Int("status", resp.HttpCode()),
		logger.Bool("is_success", resp.IsSuccess()),
		logger.Bool("is_error", isErr),
	)
}

// ── 12. Streaming download (WithOutput) ───────────────────────────────────────
//
// WithOutput(w) streams the response body directly to w without buffering.
// After Execute(), Response.Body is nil — never call Consume() or SaveToFile()
// on the same response.
func exampleStreamingDownload() {
	log := helper.GetLogger().WithContext(context.Background())

	// Stream directly to a temp file.
	f, err := os.CreateTemp("", "download-*.bin")
	if err != nil {
		log.Error("create temp file failed", logger.Error(err))
		return
	}
	defer os.Remove(f.Name())
	defer f.Close()

	resp := httpclient.Get("https://httpbin.org/stream-bytes/1024").
		WithOutput(f).
		Execute()
	if resp.Error != nil {
		log.Error("streaming download failed", logger.Error(resp.Error))
		return
	}
	info, _ := f.Stat()
	log.Info("streaming download",
		logger.String("file", f.Name()),
		logger.Int64("bytes", info.Size()),
	)
}

// ── 13. Context cancellation ──────────────────────────────────────────────────
//
// WithContext sets the request context. Cancellation or timeout propagates
// through the middleware stack and into the underlying http.Client.
func exampleContextCancellation() {
	log := helper.GetLogger().WithContext(context.Background())

	// Short timeout — /delay/2 sleeps 2 seconds, we cancel after 500ms.
	reqCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	resp := httpclient.Get("https://httpbin.org/delay/2").
		WithContext(reqCtx).
		Execute()
	if resp.Error != nil {
		log.Info("context cancellation: request cancelled as expected",
			logger.String("error", resp.Error.Error()))
		return
	}
	log.Warn("expected cancellation but request succeeded")
}

// ── 14. Retry middleware ──────────────────────────────────────────────────────
//
// RetryMiddleware retries on configurable conditions.
//   - RetryOnError: only on transport/network errors (resp is nil)
//   - RetryOnStatus(codes...): only on specific HTTP status codes
//   - RetryOnAny: on transport errors and any 5xx response
//
// Backoff strategies:
//   - FixedBackoff(d): same delay every time
//   - ExponentialBackoff(base, multiplier): geometric growth per attempt
//   - ExponentialWithJitter(base, multiplier): exponential + random jitter
//     (preferred in production — avoids thundering herd on simultaneous retries)
func exampleRetryMiddleware() {
	log := helper.GetLogger().WithContext(context.Background())

	// Client that retries up to 3 times on 429 or 5xx,
	// with exponential backoff + jitter starting at 100ms.
	client := httpclient.New(
		httpclient.WithBaseURL("https://httpbin.org"),
		httpclient.WithDefaultTimeout(15*time.Second),
		httpclient.WithMiddleware(httpclient.RetryMiddleware(httpclient.RetryConfig{
			MaxAttempts: 3,
			Backoff:     httpclient.ExponentialWithJitter(100*time.Millisecond, 2.0),
			RetryOn:     httpclient.RetryOnStatus(429, 500, 502, 503, 504),
		})),
	)

	type Result struct {
		Origin string `json:"origin"`
	}
	var result Result

	// /get always returns 200, so only 1 attempt is made.
	err := client.Get("/get").Execute().Consume(&result)
	if err != nil {
		log.Error("retry example failed", logger.Error(err))
		return
	}
	log.Info("retry middleware: succeeded", logger.String("origin", result.Origin))
}

// ── 15. Circuit breaker ───────────────────────────────────────────────────────
//
// CircuitBreakerMiddleware is global per host — state is shared across all
// requests to the same scheme://host, regardless of which Client or builder
// created the request.
//
// State machine:
//
//	CLOSED → OPEN (after FailureThreshold failures)
//	       → HALF_OPEN (after RecoveryTimeout)
//	       → CLOSED (on first success in HALF_OPEN)
//
// 5xx responses and transport errors count as failures.
// 4xx and 2xx responses count as successes.
//
// Use WithoutMiddleware(CircuitBreakerKey) to skip the CB for a specific request
// (e.g., a health-check probe that must always go through).
func exampleCircuitBreaker() {
	log := helper.GetLogger().WithContext(context.Background())

	client := httpclient.New(
		httpclient.WithBaseURL("https://httpbin.org"),
		httpclient.WithMiddleware(httpclient.CircuitBreakerMiddleware(
			httpclient.CircuitBreakerConfig{
				FailureThreshold: 3,
				RecoveryTimeout:  10 * time.Second,
			},
		)),
	)

	type Result struct {
		URL string `json:"url"`
	}

	// Normal request — goes through the circuit breaker.
	var result Result
	err := client.Get("/get").Execute().Consume(&result)
	if err != nil {
		log.Error("circuit breaker normal request failed", logger.Error(err))
		return
	}
	log.Info("circuit breaker: normal request", logger.String("url", result.URL))

	// Health check that bypasses the circuit breaker entirely.
	var health Result
	err = client.Get("/get").
		WithoutMiddleware(httpclient.CircuitBreakerKey).
		Execute().
		Consume(&health)
	if err != nil {
		log.Error("circuit breaker health check failed", logger.Error(err))
		return
	}
	log.Info("circuit breaker: health check bypassed CB", logger.String("url", health.URL))
}

// ── 16. Server-Sent Events (SSE) ─────────────────────────────────────────────
//
// ExecuteSSE reads a text/event-stream response per RFC 8895.
// The callback is called for each complete event. Return a non-nil error to stop.
// Context cancellation propagates into the stream reader.
//
// SSEEvent fields:
//   - Event: event type (defaults to "message" when the server omits the field)
//   - Data:  payload (multi-line data fields are joined with \n)
//   - ID:    last-event-ID hint for client reconnection
//   - Retry: server reconnection hint in milliseconds
func exampleSSE() {
	log := helper.GetLogger().WithContext(context.Background())

	// Use a timeout context so the example doesn't block forever.
	sseCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const maxEvents = 3
	count := 0

	err := httpclient.Get("https://httpbin.org/events/sse").
		WithContext(sseCtx).
		ExecuteSSE(func(e httpclient.SSEEvent) error {
			count++
			log.Info("SSE event received",
				logger.Int("n", count),
				logger.String("event", e.Event),
				logger.String("data", e.Data),
				logger.String("id", e.ID),
			)
			if count >= maxEvents {
				// Returning an error stops the stream.
				return fmt.Errorf("received %d events, stopping", maxEvents)
			}
			return nil
		})

	// A stop error from our own callback is expected — don't treat it as failure.
	if err != nil && count < maxEvents {
		log.Error("SSE stream error", logger.Error(err))
		return
	}
	log.Info("SSE complete", logger.Int("events_received", count))
}
