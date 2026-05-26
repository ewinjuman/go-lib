// httpclient/request.go
package httpclient

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"github.com/ewinjuman/go-lib/v2/logger"
)

// Method represents an HTTP verb.
type Method string

const (
	MethodPost    Method = "POST"
	MethodGet     Method = "GET"
	MethodPut     Method = "PUT"
	MethodDelete  Method = "DELETE"
	MethodPatch   Method = "PATCH"
	MethodOptions Method = "OPTIONS"
)

func (m Method) String() string { return string(m) }

type bodyMode int

const (
	bodyModeNone      bodyMode = iota
	bodyModeJSON
	bodyModeForm
	bodyModeMultipart
	bodyModeRaw
)

// MultipartPart is one part of a multipart/form-data request body.
type MultipartPart struct {
	field    string
	filename string
	reader   io.Reader // non-nil for FileFromReader; nil for FileFromPath (opened lazily)
	value    string    // text value for fields; file path for FileFromPath
	isFile   bool
}

// Field creates a text form field for a multipart request.
func Field(key, value string) MultipartPart {
	return MultipartPart{field: key, value: value}
}

// FileFromReader creates a file part using an existing io.Reader.
func FileFromReader(field, filename string, r io.Reader) MultipartPart {
	return MultipartPart{field: field, filename: filename, reader: r, isFile: true}
}

// FileFromPath creates a file part whose content is read from disk during Execute.
func FileFromPath(field, path string) MultipartPart {
	return MultipartPart{field: field, filename: path, value: path, isFile: true}
}

// RequestBuilder holds per-request configuration on top of a Client's shared config.
// Not safe for concurrent use — create a new builder for each request.
// Created via Client.Post/Get/etc., not directly.
type RequestBuilder struct {
	client         *Client
	method         Method
	path           string
	body           any
	bodyMode       bodyMode
	rawBody        []byte
	rawContentType string
	multipartParts []MultipartPart
	pathParams     map[string]string
	queryParams    map[string]string
	headers        http.Header
	cookies        []*http.Cookie
	ctx            context.Context
	timeout        time.Duration
	output         io.Writer
	successCodes   []int
	requestID      string
	debug          bool
	skipTLS        bool
	skipMiddleware map[string]bool
	writer         logger.Writer
}

// newRequestBuilder creates a RequestBuilder initialised from the client's shared config.
func newRequestBuilder(c *Client, method Method, path string) *RequestBuilder {
	return &RequestBuilder{
		client:       c,
		method:       method,
		path:         path,
		headers:      c.defaultHeaders.Clone(),
		successCodes: []int{200},
	}
}

// WithBody sets the request body as JSON.
func (rb *RequestBuilder) WithBody(v any) *RequestBuilder {
	rb.body = v
	rb.bodyMode = bodyModeJSON
	return rb
}

// WithForm sets the request body as form-encoded data.
func (rb *RequestBuilder) WithForm(v any) *RequestBuilder {
	rb.body = v
	rb.bodyMode = bodyModeForm
	return rb
}

// WithMultipart sets the request body as multipart/form-data with the given parts.
func (rb *RequestBuilder) WithMultipart(parts ...MultipartPart) *RequestBuilder {
	rb.multipartParts = parts
	rb.bodyMode = bodyModeMultipart
	return rb
}

// WithRawBody sets an arbitrary raw body with the given Content-Type.
func (rb *RequestBuilder) WithRawBody(b []byte, contentType string) *RequestBuilder {
	rb.rawBody = b
	rb.rawContentType = contentType
	rb.bodyMode = bodyModeRaw
	return rb
}

// WithGraphQL sets a JSON body {"query": q, "variables": vars}.
func (rb *RequestBuilder) WithGraphQL(q string, vars map[string]any) *RequestBuilder {
	rb.body = map[string]any{"query": q, "variables": vars}
	rb.bodyMode = bodyModeJSON
	return rb
}

// WithQueryParams sets the query string parameters for this request.
func (rb *RequestBuilder) WithQueryParams(params map[string]string) *RequestBuilder {
	rb.queryParams = params
	return rb
}

// WithQueryParam is an alias for WithQueryParams kept for backward compatibility.
func (rb *RequestBuilder) WithQueryParam(params map[string]string) *RequestBuilder {
	return rb.WithQueryParams(params)
}

// WithPathParam replaces :key placeholders in the URL path.
func (rb *RequestBuilder) WithPathParam(params map[string]string) *RequestBuilder {
	rb.pathParams = params
	return rb
}

// WithHeaders merges the given headers into the request headers.
func (rb *RequestBuilder) WithHeaders(headers map[string]string) *RequestBuilder {
	for k, v := range headers {
		rb.headers.Set(k, v)
	}
	return rb
}

// WithBearer sets the Authorization header to "Bearer <token>".
func (rb *RequestBuilder) WithBearer(token string) *RequestBuilder {
	rb.headers.Set("Authorization", "Bearer "+token)
	return rb
}

// WithBasicAuth sets the Authorization header using HTTP Basic auth encoding.
func (rb *RequestBuilder) WithBasicAuth(user, pass string) *RequestBuilder {
	token := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	rb.headers.Set("Authorization", "Basic "+token)
	return rb
}

// WithCookie appends a cookie to the request.
func (rb *RequestBuilder) WithCookie(name, val string) *RequestBuilder {
	rb.cookies = append(rb.cookies, &http.Cookie{Name: name, Value: val})
	return rb
}

// WithTimeout overrides the client's default timeout for this request only.
func (rb *RequestBuilder) WithTimeout(d time.Duration) *RequestBuilder {
	rb.timeout = d
	return rb
}

// WithOutput streams the response body directly to w instead of buffering in Response.Body.
// After Execute(), Response.Body is nil — do not call Consume() or SaveToFile() on the result.
func (rb *RequestBuilder) WithOutput(w io.Writer) *RequestBuilder {
	rb.output = w
	return rb
}

// WithoutMiddleware skips the named middlewares for this request only.
// Pass CircuitBreakerKey, RetryKey, LoggingKey, or any custom key.
func (rb *RequestBuilder) WithoutMiddleware(keys ...string) *RequestBuilder {
	if rb.skipMiddleware == nil {
		rb.skipMiddleware = make(map[string]bool)
	}
	for _, k := range keys {
		rb.skipMiddleware[k] = true
	}
	return rb
}

// WithSuccessCodes overrides the default success codes (200) for this request.
func (rb *RequestBuilder) WithSuccessCodes(codes []int) *RequestBuilder {
	if len(codes) > 0 {
		rb.successCodes = codes
	}
	return rb
}

// WithRequestID sets the request ID propagated in X-REQUEST-ID header.
func (rb *RequestBuilder) WithRequestID(id string) *RequestBuilder {
	rb.requestID = id
	return rb
}

// WithDebug enables or disables debug logging for this request.
func (rb *RequestBuilder) WithDebug(debug bool) *RequestBuilder {
	rb.debug = debug
	return rb
}

// WithSkipTLS disables TLS certificate verification for this request only.
func (rb *RequestBuilder) WithSkipTLS() *RequestBuilder {
	rb.skipTLS = true
	return rb
}

// WithContext sets a context for this request, overriding the background context.
func (rb *RequestBuilder) WithContext(ctx context.Context) *RequestBuilder {
	rb.ctx = ctx
	return rb
}

// WithWriter sets a custom logger.Writer for this request.
func (rb *RequestBuilder) WithWriter(w logger.Writer) *RequestBuilder {
	rb.writer = w
	return rb
}

// Execute sends the HTTP request. Defined in execute.go.
// This stub will be replaced in Task 6.
func (rb *RequestBuilder) Execute() *Response {
	panic("httpclient: Execute not yet implemented — ensure execute.go is compiled")
}
