// httpclient/client_test.go
package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_defaults(t *testing.T) {
	c := New()
	assert.NotNil(t, c)
	assert.Empty(t, c.baseURL)
	assert.Equal(t, time.Duration(0), c.defaultTimeout)
	assert.NotNil(t, c.defaultHeaders)
}

func TestNew_withBaseURL(t *testing.T) {
	c := New(WithBaseURL("https://api.example.com"))
	assert.Equal(t, "https://api.example.com", c.baseURL)
}

func TestNew_withDefaultTimeout(t *testing.T) {
	c := New(WithDefaultTimeout(5 * time.Second))
	assert.Equal(t, 5*time.Second, c.defaultTimeout)
}

func TestNew_withDefaultHeaders(t *testing.T) {
	c := New(WithDefaultHeaders(map[string]string{"X-App": "test"}))
	assert.Equal(t, "test", c.defaultHeaders.Get("X-App"))
}

func TestNew_withMiddleware_appendsToSlice(t *testing.T) {
	m := NewMiddleware("test", func(next Doer) Doer { return next })
	c := New(WithMiddleware(m))
	assert.Len(t, c.middlewares, 1)
}

func TestNew_withDefaultBearer_storesFn(t *testing.T) {
	fn := func() string { return "tok" }
	c := New(WithDefaultBearer(fn))
	assert.NotNil(t, c.defaultBearer)
	assert.Equal(t, "tok", c.defaultBearer())
}

func TestClient_buildHTTPClient_setsTimeout(t *testing.T) {
	c := New()
	hc := c.buildHTTPClient(false, 3*time.Second)
	assert.Equal(t, 3*time.Second, hc.Timeout)
}

func TestClient_buildHTTPClient_zeroTimeout_noTimeout(t *testing.T) {
	c := New()
	hc := c.buildHTTPClient(false, 0)
	assert.Equal(t, time.Duration(0), hc.Timeout)
}

func TestClient_buildHTTPClient_withCookieJar(t *testing.T) {
	c := New(WithCookieJar(http.DefaultClient.Jar))
	hc := c.buildHTTPClient(false, 0)
	assert.Equal(t, http.DefaultClient.Jar, hc.Jar)
}

// require is used to ensure no unused import error when future tests are added.
var _ = require.New
