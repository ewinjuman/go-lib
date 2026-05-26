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
