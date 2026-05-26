// httpclient/client.go
package httpclient

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client holds shared configuration and the middleware stack for a group of requests.
// Create one per upstream service and reuse it across requests — it is safe for concurrent use
// after construction (all fields are set once during New() and never mutated).
type Client struct {
	baseURL          string
	defaultHeaders   http.Header
	defaultTimeout   time.Duration
	defaultBearer    func() string // called per request to get the current token; nil if not set
	defaultBasicUser string
	defaultBasicPass string
	middlewares      []Middleware
	cookieJar        http.CookieJar

	// Transport knobs — stored individually so they compose without clobbering each other.
	// WithTransport sets transport directly and takes precedence over all three knobs.
	skipTLS   bool           // set by WithSkipTLS; per-request skipTLS OR-ed in buildHTTPClient
	tlsCfg    *tls.Config    // set by WithTLSConfig; ignored when skipTLS is true
	proxyURL  string         // set by WithProxy; validated at option creation time
	transport http.RoundTripper // full override; if non-nil, knobs above are ignored
}

// ClientOption configures a Client during New().
type ClientOption func(*Client)

// New creates a Client with the given options.
// The returned Client is safe for concurrent use after construction.
func New(opts ...ClientOption) *Client {
	c := &Client{defaultHeaders: http.Header{}}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithBaseURL sets a base URL prepended to all relative request paths.
func WithBaseURL(base string) ClientOption {
	return func(c *Client) { c.baseURL = base }
}

// WithDefaultTimeout sets the timeout used for requests that don't set their own.
// Zero means no timeout.
func WithDefaultTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.defaultTimeout = d }
}

// WithDefaultHeaders merges headers sent with every request from this client.
// Existing headers with the same name are overwritten.
func WithDefaultHeaders(headers map[string]string) ClientOption {
	return func(c *Client) {
		for k, v := range headers {
			c.defaultHeaders.Set(k, v)
		}
	}
}

// WithDefaultBearer sets a token provider called per request.
// fn is called on every Execute() — return a fresh token if the token rotates.
// Ignored when the request already has an Authorization header.
func WithDefaultBearer(fn func() string) ClientOption {
	return func(c *Client) { c.defaultBearer = fn }
}

// WithDefaultBasicAuth sets Basic auth credentials sent with every request.
// Ignored when the request already has an Authorization header.
func WithDefaultBasicAuth(user, pass string) ClientOption {
	return func(c *Client) {
		c.defaultBasicUser = user
		c.defaultBasicPass = pass
	}
}

// WithMiddleware appends a middleware to the transport stack.
// Middlewares are applied in registration order: first registered = outermost.
func WithMiddleware(m Middleware) ClientOption {
	return func(c *Client) { c.middlewares = append(c.middlewares, m) }
}

// WithCookieJar attaches a cookie jar — cookies are sent and stored automatically.
func WithCookieJar(jar http.CookieJar) ClientOption {
	return func(c *Client) { c.cookieJar = jar }
}

// WithSkipTLS disables TLS certificate verification for all requests from this client.
// Use only in development/testing — never in production.
// When combined with WithTLSConfig, WithSkipTLS takes precedence regardless of option order.
func WithSkipTLS() ClientOption {
	return func(c *Client) { c.skipTLS = true }
}

// WithTLSConfig sets a custom TLS configuration for all requests from this client.
// Ignored when WithSkipTLS is also set (InsecureSkipVerify takes precedence).
func WithTLSConfig(cfg *tls.Config) ClientOption {
	return func(c *Client) { c.tlsCfg = cfg }
}

// WithProxy sets an HTTP proxy URL for all requests from this client.
// Panics at construction time if proxyURL is not a valid absolute URL with a host
// (e.g. "http://proxy:8080"). Composable with WithSkipTLS and WithTLSConfig.
func WithProxy(proxyURL string) ClientOption {
	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed.Host == "" {
		panic(fmt.Sprintf("httpclient: invalid proxy URL %q: host must not be empty", proxyURL))
	}
	return func(c *Client) { c.proxyURL = proxyURL }
}

// WithTransport replaces the underlying http.RoundTripper.
// Useful for testing with a mock transport or adding custom connection pooling.
func WithTransport(t http.RoundTripper) ClientOption {
	return func(c *Client) { c.transport = t }
}

// buildHTTPClient constructs a net/http.Client for a single Execute() call.
//
// Transport resolution order (highest priority first):
//  1. c.transport (WithTransport) — full override; knobs below are ignored.
//  2. Composed *http.Transport from c.skipTLS||perRequestSkipTLS, c.tlsCfg, c.proxyURL.
//  3. nil → net/http uses http.DefaultTransport.
//
// perRequestSkipTLS is true when the RequestBuilder called WithSkipTLS() for this request.
// timeout is applied only when > 0.
func (c *Client) buildHTTPClient(perRequestSkipTLS bool, timeout time.Duration) *http.Client {
	var transport http.RoundTripper
	if c.transport != nil {
		transport = c.transport
	} else {
		wantCustomTransport := c.skipTLS || perRequestSkipTLS || c.tlsCfg != nil || c.proxyURL != ""
		if wantCustomTransport {
			t := &http.Transport{}
			if c.skipTLS || perRequestSkipTLS {
				t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // intentional for dev/test
			} else if c.tlsCfg != nil {
				t.TLSClientConfig = c.tlsCfg
			}
			if c.proxyURL != "" {
				parsed, _ := url.Parse(c.proxyURL) // already validated in WithProxy
				t.Proxy = http.ProxyURL(parsed)
			}
			transport = t
		}
	}
	hc := &http.Client{Jar: c.cookieJar, Transport: transport}
	if timeout > 0 {
		hc.Timeout = timeout
	}
	return hc
}

// Post creates a RequestBuilder for a POST request to path.
func (c *Client) Post(path string) *RequestBuilder { return c.newBuilder(MethodPost, path) }

// Get creates a RequestBuilder for a GET request to path.
func (c *Client) Get(path string) *RequestBuilder { return c.newBuilder(MethodGet, path) }

// Put creates a RequestBuilder for a PUT request to path.
func (c *Client) Put(path string) *RequestBuilder { return c.newBuilder(MethodPut, path) }

// Delete creates a RequestBuilder for a DELETE request to path.
func (c *Client) Delete(path string) *RequestBuilder { return c.newBuilder(MethodDelete, path) }

// Patch creates a RequestBuilder for a PATCH request to path.
func (c *Client) Patch(path string) *RequestBuilder { return c.newBuilder(MethodPatch, path) }

// Options creates a RequestBuilder for an OPTIONS request to path.
func (c *Client) Options(path string) *RequestBuilder { return c.newBuilder(MethodOptions, path) }

// newBuilder is the internal factory; replaced by the real implementation in request.go (Task 4).
func (c *Client) newBuilder(method Method, path string) *RequestBuilder {
	panic("httpclient: internal error: newBuilder not yet wired — ensure request.go is compiled")
}
