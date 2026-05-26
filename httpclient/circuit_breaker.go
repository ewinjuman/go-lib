package httpclient

import (
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	Error "github.com/ewinjuman/go-lib/v2/apperror"
)

var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrAppsCircuitOpen *Error.ApplicationError
	cbRegistry         sync.Map // map[string]*CircuitBreaker, keyed by scheme://host
)

func init() {
	ErrAppsCircuitOpen = Error.NewError(123, "FAILED", "circuit breaker is open").(*Error.ApplicationError).WithCause(ErrCircuitOpen)
}

// CircuitBreakerConfig holds tunable parameters for a circuit breaker.
// Zero values fall back to package defaults.
type CircuitBreakerConfig struct {
	FailureThreshold     int           // failures before opening (default: 5)
	RecoveryTimeout      time.Duration // wait before half-open probe (default: 1m)
	FailureRateThreshold float64       // failure ratio 0-1 to open circuit (default: 0.5)
}

// getCircuitBreaker returns the circuit breaker for the host derived from rawURL.
// cfg is applied only when a circuit breaker is created for the first time for that host.
func getCircuitBreaker(rawURL string, cfg *CircuitBreakerConfig) *CircuitBreaker {
	host := extractHost(rawURL)
	cb, _ := cbRegistry.LoadOrStore(host, newCircuitBreaker(cfg))
	return cb.(*CircuitBreaker)
}

func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Scheme + "://" + u.Host
}

type CircuitBreaker struct {
	mu                   sync.Mutex
	state                string
	failureCount         int
	lastFailureTime      time.Time
	failureThreshold     int
	recoveryTimeout      time.Duration
	totalRequestCount    int
	failureRateThreshold float64
}

func newCircuitBreaker(cfg *CircuitBreakerConfig) *CircuitBreaker {
	cb := &CircuitBreaker{
		state:                "CLOSED",
		failureThreshold:     5,
		recoveryTimeout:      1 * time.Minute,
		failureRateThreshold: 0.5,
	}
	if cfg == nil {
		return cb
	}
	if cfg.FailureThreshold > 0 {
		cb.failureThreshold = cfg.FailureThreshold
	}
	if cfg.RecoveryTimeout > 0 {
		cb.recoveryTimeout = cfg.RecoveryTimeout
	}
	if cfg.FailureRateThreshold > 0 && cfg.FailureRateThreshold <= 1 {
		cb.failureRateThreshold = cfg.FailureRateThreshold
	}
	return cb
}

// NewCircuitBreaker creates a circuit breaker with default settings.
func NewCircuitBreaker() *CircuitBreaker {
	return newCircuitBreaker(nil)
}

func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == "OPEN" {
		if time.Since(cb.lastFailureTime) < cb.recoveryTimeout {
			return ErrAppsCircuitOpen
		}
		// Transition to HALF_OPEN: reset counters to evaluate fresh
		cb.state = "HALF_OPEN"
		cb.failureCount = 0
		cb.totalRequestCount = 0
	}

	return nil
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequestCount++

	if cb.state == "HALF_OPEN" {
		cb.state = "CLOSED"
		cb.failureCount = 0
		cb.totalRequestCount = 0
		return
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequestCount++
	cb.failureCount++

	if cb.state == "HALF_OPEN" {
		cb.state = "OPEN"
		cb.lastFailureTime = time.Now()
		return
	}

	failureRate := float64(cb.failureCount) / float64(cb.totalRequestCount)
	if cb.failureCount >= cb.failureThreshold &&
		(cb.failureRateThreshold <= 0 || failureRate >= cb.failureRateThreshold) {
		cb.state = "OPEN"
		cb.lastFailureTime = time.Now()
	}
}

func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

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
