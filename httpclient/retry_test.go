// httpclient/retry_test.go
package httpclient

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixedBackoff_alwaysReturnsSameDelay(t *testing.T) {
	bf := FixedBackoff(100 * time.Millisecond)
	assert.Equal(t, 100*time.Millisecond, bf(0))
	assert.Equal(t, 100*time.Millisecond, bf(5))
}

func TestExponentialBackoff_doublesEachAttempt(t *testing.T) {
	bf := ExponentialBackoff(100*time.Millisecond, 2.0)
	assert.Equal(t, 100*time.Millisecond, bf(0))
	assert.Equal(t, 200*time.Millisecond, bf(1))
	assert.Equal(t, 400*time.Millisecond, bf(2))
}

func TestExponentialWithJitter_withinExpectedRange(t *testing.T) {
	bf := ExponentialWithJitter(100*time.Millisecond, 2.0)
	for i := 0; i < 20; i++ {
		// attempt=1 → base=200ms; jitter adds 0–200ms → total 0–400ms
		d := bf(1)
		assert.GreaterOrEqual(t, int64(d), int64(0))
		assert.LessOrEqual(t, int64(d), int64(400*time.Millisecond))
	}
}

func TestRetryOnError_retriesOnlyOnNetworkError(t *testing.T) {
	assert.True(t, RetryOnError(nil, errors.New("dial tcp")))
	assert.False(t, RetryOnError(&http.Response{StatusCode: 500}, nil))
}

func TestRetryOnStatus_retriesOnMatchingStatus(t *testing.T) {
	fn := RetryOnStatus(429, 503)
	assert.True(t, fn(&http.Response{StatusCode: 429}, nil))
	assert.True(t, fn(&http.Response{StatusCode: 503}, nil))
	assert.False(t, fn(&http.Response{StatusCode: 200}, nil))
	assert.False(t, fn(nil, errors.New("network")))
}

func TestRetryOnAny_retriesOnErrorOrServerError(t *testing.T) {
	assert.True(t, RetryOnAny(nil, errors.New("err")))
	assert.True(t, RetryOnAny(&http.Response{StatusCode: 503}, nil))
	assert.False(t, RetryOnAny(&http.Response{StatusCode: 200}, nil))
}

func TestRetryMiddleware_retriesUntilSuccess(t *testing.T) {
	var calls atomic.Int32
	base := DoerFunc(func(r *http.Request) (*http.Response, error) {
		n := calls.Add(1)
		if n < 3 {
			return nil, errors.New("temporary failure")
		}
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	})
	m := RetryMiddleware(RetryConfig{
		MaxAttempts: 3,
		Backoff:     FixedBackoff(0),
		RetryOn:     RetryOnError,
	})
	composed := Apply(base, []Middleware{m}, nil)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://example.com", nil)
	resp, err := composed.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, int32(3), calls.Load())
}

func TestRetryMiddleware_doesNotRetryOnSuccess(t *testing.T) {
	var calls atomic.Int32
	base := DoerFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	})
	m := RetryMiddleware(RetryConfig{MaxAttempts: 3, Backoff: FixedBackoff(0), RetryOn: RetryOnError})
	composed := Apply(base, []Middleware{m}, nil)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, _ := composed.Do(req)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, int32(1), calls.Load())
}

func TestRetryMiddleware_stopsOnContextCancellation(t *testing.T) {
	var calls atomic.Int32
	base := DoerFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("fail")
	})
	m := RetryMiddleware(RetryConfig{
		MaxAttempts: 10,
		Backoff:     FixedBackoff(50 * time.Millisecond),
		RetryOn:     RetryOnError,
	})
	composed := Apply(base, []Middleware{m}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)
	_, err := composed.Do(req)
	assert.Error(t, err)
	assert.Less(t, calls.Load(), int32(10))
}
