// httpclient/retry.go
package httpclient

import (
	"math"
	"math/rand"
	"net/http"
	"time"
)

// BackoffFunc returns the delay before the nth retry attempt (0-indexed).
type BackoffFunc func(attempt int) time.Duration

// FixedBackoff waits d before every retry.
func FixedBackoff(d time.Duration) BackoffFunc {
	return func(_ int) time.Duration { return d }
}

// ExponentialBackoff returns base * multiplier^attempt.
func ExponentialBackoff(base time.Duration, multiplier float64) BackoffFunc {
	return func(attempt int) time.Duration {
		return time.Duration(float64(base) * math.Pow(multiplier, float64(attempt)))
	}
}

// ExponentialWithJitter adds uniform random jitter in [0, delay] to ExponentialBackoff.
// This prevents thundering-herd effects when many clients retry simultaneously.
func ExponentialWithJitter(base time.Duration, multiplier float64) BackoffFunc {
	exp := ExponentialBackoff(base, multiplier)
	return func(attempt int) time.Duration {
		d := exp(attempt)
		jitter := time.Duration(rand.Int63n(int64(d) + 1))
		return d + jitter
	}
}

// RetryOnError retries only on transport/network errors (resp is nil).
func RetryOnError(resp *http.Response, err error) bool { return err != nil }

// RetryOnStatus returns a condition that retries when the response status is in codes.
// Does not retry on transport errors.
func RetryOnStatus(codes ...int) func(*http.Response, error) bool {
	set := make(map[int]bool, len(codes))
	for _, c := range codes {
		set[c] = true
	}
	return func(resp *http.Response, err error) bool {
		if err != nil || resp == nil {
			return false
		}
		return set[resp.StatusCode]
	}
}

// RetryOnAny retries on transport errors and on any 5xx response.
func RetryOnAny(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp != nil && resp.StatusCode >= 500
}

// RetryConfig configures the RetryMiddleware.
type RetryConfig struct {
	MaxAttempts int                                        // total attempts including the first; ≤0 means 1
	Backoff     BackoffFunc                                // delay before each retry; nil = no delay
	RetryOn     func(resp *http.Response, err error) bool // nil = never retry
}

// RetryMiddleware retries failed requests according to cfg.
// Context cancellation stops the retry loop immediately.
func RetryMiddleware(cfg RetryConfig) Middleware {
	return NewMiddleware(RetryKey, func(next Doer) Doer {
		return DoerFunc(func(req *http.Request) (*http.Response, error) {
			max := cfg.MaxAttempts
			if max <= 0 {
				max = 1
			}
			var (
				resp *http.Response
				err  error
			)
			for attempt := 0; attempt < max; attempt++ {
				select {
				case <-req.Context().Done():
					return nil, req.Context().Err()
				default:
				}

				resp, err = next.Do(req)

				if attempt == max-1 || cfg.RetryOn == nil || !cfg.RetryOn(resp, err) {
					return resp, err
				}

				// Drain body before retry to allow connection reuse
				if resp != nil && resp.Body != nil {
					resp.Body.Close()
				}

				if cfg.Backoff != nil {
					delay := cfg.Backoff(attempt)
					if delay > 0 {
						timer := time.NewTimer(delay)
						select {
						case <-req.Context().Done():
							timer.Stop()
							return nil, req.Context().Err()
						case <-timer.C:
						}
					}
				}
			}
			return resp, err
		})
	})
}
