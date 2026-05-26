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
