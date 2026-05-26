package httpclient

import (
	"errors"
	"net/http"
	"testing"
	"time"

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
	// Use a 1ms RecoveryTimeout so the test doesn't flake but transitions are fast.
	cb := newCircuitBreaker(&CircuitBreakerConfig{FailureThreshold: 1, RecoveryTimeout: 1})
	cb.RecordFailure()
	require.Equal(t, "OPEN", cb.State())
	// Wait for recovery timeout to elapse, then Allow() transitions to HALF_OPEN.
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, cb.Allow())
	assert.Equal(t, "HALF_OPEN", cb.State())
	cb.RecordSuccess()
	assert.Equal(t, "CLOSED", cb.State())
}

func TestCircuitBreakerMiddleware_blocksWhenCircuitIsOpen(t *testing.T) {
	cbRegistry.Delete("http://blocked-host")

	// Trip the circuit via the middleware using a 5xx response (FailureThreshold: 1).
	tripBase := DoerFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: http.NoBody}, nil
	})
	m := CircuitBreakerMiddleware(CircuitBreakerConfig{FailureThreshold: 1})
	tripComposed := Apply(tripBase, []Middleware{m}, nil)
	req, _ := http.NewRequest("GET", "http://blocked-host/path", nil)
	tripComposed.Do(req) // records a failure; circuit opens (failureCount=1 >= threshold=1, rate=1.0 >= 0.5)

	// Now verify the circuit is open and blocks a subsequent request.
	called := false
	successBase := DoerFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	})
	composed := Apply(successBase, []Middleware{m}, nil)
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
	for range 5 {
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
