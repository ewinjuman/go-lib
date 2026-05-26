package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ewinjuman/go-lib/v2/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseDoer(status int) Doer {
	return DoerFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: http.NoBody}, nil
	})
}

func TestDoerFunc_implementsDoer(t *testing.T) {
	df := DoerFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 204, Body: http.NoBody}, nil
	})
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	resp, err := df.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 204, resp.StatusCode)
}

func TestApply_firstMiddlewareIsOutermost(t *testing.T) {
	var order []string
	m1 := NewMiddleware("m1", func(next Doer) Doer {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			order = append(order, "m1-req")
			resp, err := next.Do(r)
			order = append(order, "m1-resp")
			return resp, err
		})
	})
	m2 := NewMiddleware("m2", func(next Doer) Doer {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			order = append(order, "m2-req")
			resp, err := next.Do(r)
			order = append(order, "m2-resp")
			return resp, err
		})
	})
	composed := Apply(baseDoer(200), []Middleware{m1, m2}, nil)
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	_, err = composed.Do(req)
	require.NoError(t, err)
	assert.Equal(t, []string{"m1-req", "m2-req", "m2-resp", "m1-resp"}, order)
}

func TestApply_skipsMiddlewareByKey(t *testing.T) {
	called := false
	m := NewMiddleware("skip-me", func(next Doer) Doer {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			called = true
			return next.Do(r)
		})
	})
	composed := Apply(baseDoer(200), []Middleware{m}, map[string]bool{"skip-me": true})
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	_, _ = composed.Do(req)
	assert.False(t, called)
}

func TestApply_nilSkipMap_callsAllMiddleware(t *testing.T) {
	called := false
	m := NewMiddleware("m", func(next Doer) Doer {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			called = true
			return next.Do(r)
		})
	})
	composed := Apply(baseDoer(200), []Middleware{m}, nil)
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	_, _ = composed.Do(req)
	assert.True(t, called)
}

func TestApply_emptyMiddlewares_returnsBase(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	resp, err := Apply(baseDoer(418), []Middleware{}, nil).Do(req)
	require.NoError(t, err)
	assert.Equal(t, 418, resp.StatusCode)
}

func TestNewMiddleware_nilFn_panics(t *testing.T) {
	assert.Panics(t, func() {
		NewMiddleware("key", nil)
	})
}

// spyWriter is a test double for logger.Writer that records all Print calls.
type spyWriter struct {
	calls []spyCall
}

type spyCall struct {
	message string
	values  []any
}

func (s *spyWriter) Print(ctx context.Context, message string, value ...any) {
	s.calls = append(s.calls, spyCall{message: message, values: value})
}

// Verify spyWriter satisfies logger.Writer at compile time.
var _ logger.Writer = (*spyWriter)(nil)

func TestLoggingMiddleware_callsPrintForRequestAndResponse(t *testing.T) {
	spy := &spyWriter{}
	mw := LoggingMiddleware(spy)

	composed := Apply(baseDoer(200), []Middleware{mw}, nil)
	req, err := http.NewRequest(http.MethodGet, "http://example.com/path", nil)
	require.NoError(t, err)

	resp, err := composed.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Expect at least two Print calls: one for http_request, one for http_response.
	require.GreaterOrEqual(t, len(spy.calls), 2, "expected at least 2 Print calls")

	reqCall := spy.calls[0]
	assert.Equal(t, "http_request", reqCall.message)
	assert.Equal(t, http.MethodGet, reqCall.values[0], "first value should be HTTP method")
	assert.Equal(t, "http://example.com/path", reqCall.values[1], "second value should be URL")

	respCall := spy.calls[1]
	assert.Equal(t, "http_response", respCall.message)
	assert.Equal(t, http.MethodGet, respCall.values[0], "first value should be HTTP method")
	assert.Equal(t, "http://example.com/path", respCall.values[1], "second value should be URL")
	assert.Equal(t, 200, respCall.values[2], "third value should be status code")
}

func TestLoggingMiddleware_logsOnError(t *testing.T) {
	spy := &spyWriter{}
	mw := LoggingMiddleware(spy)

	sentinelErr := errors.New("connection refused")
	errDoer := DoerFunc(func(r *http.Request) (*http.Response, error) {
		return nil, sentinelErr
	})

	composed := Apply(errDoer, []Middleware{mw}, nil)
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	_, err = composed.Do(req)
	assert.ErrorIs(t, err, sentinelErr)

	// Should have logged the request and the error response.
	require.GreaterOrEqual(t, len(spy.calls), 2, "expected at least 2 Print calls on error path")
	assert.Equal(t, "http_request", spy.calls[0].message)
	assert.Equal(t, "http_response", spy.calls[1].message)
}

// trackingCloser wraps an io.ReadCloser and records whether Close was called.
type trackingCloser struct {
	io.ReadCloser
	closed *bool
}

func (tc *trackingCloser) Close() error {
	*tc.closed = true
	return tc.ReadCloser.Close()
}

// TestLoggingMiddleware_closesBodyOnNonNilRespAndError verifies the net/http contract:
// when next.Do returns (non-nil resp, non-nil err), LoggingMiddleware closes resp.Body
// to prevent a resource leak before discarding the response.
func TestLoggingMiddleware_closesBodyOnNonNilRespAndError(t *testing.T) {
	spy := &spyWriter{}
	mw := LoggingMiddleware(spy)

	closed := false
	trackingBody := &trackingCloser{
		ReadCloser: io.NopCloser(strings.NewReader("partial body")),
		closed:     &closed,
	}

	sentinelErr := errors.New("redirect error")
	errDoer := DoerFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 301, Body: trackingBody}, sentinelErr
	})

	composed := Apply(errDoer, []Middleware{mw}, nil)
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	_, err = composed.Do(req)
	assert.ErrorIs(t, err, sentinelErr)
	assert.True(t, closed, "resp.Body must be closed to prevent resource leak")
}
