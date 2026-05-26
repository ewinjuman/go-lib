// httpclient/client_test.go
package httpclient

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
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

func TestNew_withDefaultBasicAuth_storesCredentials(t *testing.T) {
	c := New(WithDefaultBasicAuth("user", "pass"))
	assert.Equal(t, "user", c.defaultBasicUser)
	assert.Equal(t, "pass", c.defaultBasicPass)
}

func TestNew_withSkipTLS_setsFlag(t *testing.T) {
	c := New(WithSkipTLS())
	assert.True(t, c.skipTLS)
	assert.Nil(t, c.transport) // transport knob stored separately, not on c.transport
}

func TestNew_withTLSConfig_storesCfg(t *testing.T) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	c := New(WithTLSConfig(cfg))
	assert.Equal(t, cfg, c.tlsCfg)
}

func TestNew_withProxy_storesURL(t *testing.T) {
	c := New(WithProxy("http://proxy.example.com:8080"))
	assert.Equal(t, "http://proxy.example.com:8080", c.proxyURL)
}

func TestNew_withProxy_invalidURL_panics(t *testing.T) {
	assert.Panics(t, func() {
		WithProxy("not-a-url-without-host")
	})
}

func TestNew_withTransport_setsTransport(t *testing.T) {
	rt := http.DefaultTransport
	c := New(WithTransport(rt))
	assert.Equal(t, rt, c.transport)
}

func TestNew_withSkipTLS_andProxy_bothCompose(t *testing.T) {
	// Both knobs stored independently — no clobbering.
	c := New(WithSkipTLS(), WithProxy("http://proxy.example.com:8080"))
	assert.True(t, c.skipTLS)
	assert.Equal(t, "http://proxy.example.com:8080", c.proxyURL)
	assert.Nil(t, c.transport) // full override not set
}

func TestNew_multipleMiddlewares_appendedInOrder(t *testing.T) {
	m1 := NewMiddleware("a", func(next Doer) Doer { return next })
	m2 := NewMiddleware("b", func(next Doer) Doer { return next })
	c := New(WithMiddleware(m1), WithMiddleware(m2))
	require.Len(t, c.middlewares, 2)
	assert.Equal(t, "a", c.middlewares[0].key)
	assert.Equal(t, "b", c.middlewares[1].key)
}

func TestClient_buildHTTPClient_skipTLS_setsInsecureTransport(t *testing.T) {
	c := New()
	hc := c.buildHTTPClient(true, 0) // per-request skipTLS
	require.NotNil(t, hc.Transport)
	tr, ok := hc.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, tr.TLSClientConfig)
	assert.True(t, tr.TLSClientConfig.InsecureSkipVerify)
}

func TestClient_buildHTTPClient_withTransport_ignoresKnobs(t *testing.T) {
	// WithTransport takes full precedence — skipTLS knob is ignored.
	custom := http.DefaultTransport
	c := New(WithTransport(custom), WithSkipTLS())
	hc := c.buildHTTPClient(false, 0)
	assert.Equal(t, custom, hc.Transport)
}

func TestClient_buildHTTPClient_withProxy_setsProxyOnTransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	c := New(WithProxy(srv.URL))
	hc := c.buildHTTPClient(false, 0)
	require.NotNil(t, hc.Transport)
	tr, ok := hc.Transport.(*http.Transport)
	require.True(t, ok)
	assert.NotNil(t, tr.Proxy)
}

func TestNew_withSkipTLS_clientLevel_buildsInsecureTransport(t *testing.T) {
	c := New(WithSkipTLS())
	hc := c.buildHTTPClient(false, 0) // no per-request skipTLS
	require.NotNil(t, hc.Transport)
	tr, ok := hc.Transport.(*http.Transport)
	require.True(t, ok)
	assert.True(t, tr.TLSClientConfig.InsecureSkipVerify)
}
