package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"github.com/ewinjuman/go-lib/v2/logger"
	"github.com/ewinjuman/go-lib/v2/utils"
	"github.com/google/uuid"
)

type Method string

const (
	MethodPost    Method = "POST"
	MethodGet     Method = "GET"
	MethodPut     Method = "PUT"
	MethodDelete  Method = "DELETE"
	MethodPatch   Method = "PATCH"
	MethodOptions Method = "OPTIONS"
)

func (v Method) String() string {
	return string(v)
}

type (
	MultipartData struct {
		Key   string
		Value string
		File  *bytes.Reader
	}

	Request struct {
		logger.Writer
		ID                    string
		URL                   string
		Method                Method
		Body                  interface{}
		File                  []MultipartData
		PathParams            map[string]string
		QueryParams           map[string]string
		Headers               http.Header
		Context               context.Context
		Timeout               time.Duration
		DebugMode             bool
		SkipTLS               bool
		DisableCircuitBreaker bool
		HTTPSuccessCode       []int
		CircuitBreakerConfig  *CircuitBreakerConfig
		Output                io.Writer
	}

	RequestBuilder struct {
		request Request
		client  *reqClient
	}
)

func Do(method Method, url string) *RequestBuilder {
	return &RequestBuilder{
		request: Request{
			URL:             url,
			Method:          method,
			Headers:         http.Header{},
			HTTPSuccessCode: []int{200},
		},
		client: httpclient(),
	}
}

func Post(url string) *RequestBuilder    { return Do(MethodPost, url) }
func Get(url string) *RequestBuilder     { return Do(MethodGet, url) }
func Put(url string) *RequestBuilder     { return Do(MethodPut, url) }
func Delete(url string) *RequestBuilder  { return Do(MethodDelete, url) }
func Patch(url string) *RequestBuilder   { return Do(MethodPatch, url) }
func Options(url string) *RequestBuilder { return Do(MethodOptions, url) }

func (rb *RequestBuilder) WithRequestID(requestID string) *RequestBuilder {
	rb.request.ID = requestID
	return rb
}

func (rb *RequestBuilder) WithDebug(debug bool) *RequestBuilder {
	rb.request.DebugMode = debug
	return rb
}

func (rb *RequestBuilder) WithWriter(writer logger.Writer) *RequestBuilder {
	rb.request.Writer = writer
	return rb
}

func (rb *RequestBuilder) WithSkipTLS() *RequestBuilder {
	rb.request.SkipTLS = true
	return rb
}

func (rb *RequestBuilder) WithQueryParam(queryParams map[string]string) *RequestBuilder {
	rb.request.QueryParams = queryParams
	return rb
}

func (rb *RequestBuilder) WithPathParam(pathParams map[string]string) *RequestBuilder {
	rb.request.PathParams = pathParams
	return rb
}

func (rb *RequestBuilder) WithHeaders(headers map[string]string) *RequestBuilder {
	for h, val := range headers {
		rb.request.Headers.Set(h, val)
	}
	return rb
}

func (rb *RequestBuilder) WithBody(body interface{}) *RequestBuilder {
	rb.request.Body = body
	return rb
}

func (rb *RequestBuilder) WithFile(file []MultipartData) *RequestBuilder {
	rb.request.File = file
	return rb
}

func (rb *RequestBuilder) WithContext(ctx context.Context) *RequestBuilder {
	rb.request.Context = ctx
	return rb
}

func (rb *RequestBuilder) WithTimeout(timeout time.Duration) *RequestBuilder {
	rb.request.Timeout = timeout
	return rb
}

func (rb *RequestBuilder) WithBasicAuth(username, password string) *RequestBuilder {
	token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	rb.request.Headers.Set("Authorization", "Basic "+token)
	return rb
}

func (rb *RequestBuilder) WithBearer(token string) *RequestBuilder {
	rb.request.Headers.Set("Authorization", "Bearer "+token)
	return rb
}

func (rb *RequestBuilder) WithSuccessCodes(codes []int) *RequestBuilder {
	if len(codes) > 0 {
		rb.request.HTTPSuccessCode = codes
	}
	return rb
}

func (rb *RequestBuilder) WithoutCircuitBreaker() *RequestBuilder {
	rb.request.DisableCircuitBreaker = true
	return rb
}

// WithCircuitBreakerConfig sets custom circuit breaker parameters for this host.
// Config is applied only on the first request to each host; subsequent requests
// reuse the existing circuit breaker and its accumulated state.
func (rb *RequestBuilder) WithCircuitBreakerConfig(cfg CircuitBreakerConfig) *RequestBuilder {
	rb.request.CircuitBreakerConfig = &cfg
	return rb
}

func (rb *RequestBuilder) WithOutput(w io.Writer) *RequestBuilder {
	rb.request.Output = w
	return rb
}

func (rb *RequestBuilder) Execute() *Response {
	rb.setDefaultWriter()
	rb.setDefaultHeaders()
	rb.setQueryParams()

	httpClient := rb.client.httpClient
	httpClient.Header = rb.request.Headers

	if rb.request.Timeout > 0 {
		httpClient.SetTimeout(rb.request.Timeout)
	}
	if rb.request.SkipTLS {
		httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	httpClient.SetDebug(rb.request.DebugMode)
	rb.client.httpClient = httpClient

	return rb.request.doRequest(rb.client)
}

func (rb *RequestBuilder) setDefaultWriter() {
	if utils.IsEmpty(rb.request.Writer) {
		rb.WithWriter(&logger.DefaultWriter{ID: rb.request.ID})
	}
}

func (rb *RequestBuilder) setDefaultHeaders() {
	if rb.request.Headers["Content-Type"] == nil && rb.request.Method != MethodGet {
		rb.request.Headers.Set("Content-Type", "application/json")
	}
	if rb.request.ID != "" {
		rb.request.Headers.Set("X-REQUEST-ID", rb.request.ID)
	} else {
		rb.request.Headers.Set("X-REQUEST-ID", uuid.New().String())
	}
}

func (rb *RequestBuilder) setQueryParams() {
	if rb.request.QueryParams != nil {
		rb.client.httpClient.SetQueryParams(rb.request.QueryParams)
	}
}
