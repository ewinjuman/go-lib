package httpclient

import (
	"net/http"
	"testing"

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
	req, _ := http.NewRequest("GET", "http://example.com", nil)
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
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, err := composed.Do(req)
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
	req, _ := http.NewRequest("GET", "http://example.com", nil)
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
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, _ = composed.Do(req)
	assert.True(t, called)
}
