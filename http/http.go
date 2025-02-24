package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/ewinjuman/go-lib/v2/logger"
	"github.com/ewinjuman/go-lib/v2/utils"
	"github.com/google/uuid"

	//"github.com/go-resty/resty/v2"
	"net/http"
	"time"
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
		TimeoutHystrix        int
		MaxConcurrentRequests int
		ErrorPercentThreshold int
		ErrNotSuccess         bool
	}

	RequestBuilder struct {
		request Request
		client  *reqClient
		//requestManager RequestManager
		//requestRetry   RequestRetry
	}
)

func Do(method Method, host, path string) *RequestBuilder {
	url := host + path
	return &RequestBuilder{
		request: Request{
			URL:     url,
			Method:  method,
			Headers: http.Header{},
		},
		client: httpclient(),
		//requestRetry:   &RequestRetryWhenTimeout{},
	}
}

func Post(host, endpoint string) *RequestBuilder {
	return Do(MethodPost, host, endpoint)
}

func Get(host, endpoint string) *RequestBuilder {
	return Do(MethodGet, host, endpoint)
}

func Put(host, endpoint string) *RequestBuilder {
	return Do(MethodPut, host, endpoint)
}

func Delete(host, endpoint string) *RequestBuilder {
	return Do(MethodDelete, host, endpoint)
}

func Patch(host, endpoint string) *RequestBuilder {
	return Do(MethodPatch, host, endpoint)
}

func Options(host, endpoint string) *RequestBuilder {
	return Do(MethodOptions, host, endpoint)
}

func (rb *RequestBuilder) SetRequestID(requestID string) *RequestBuilder {
	rb.request.ID = requestID
	return rb
}

func (rb *RequestBuilder) SetDebug(debug bool) *RequestBuilder {
	rb.request.DebugMode = debug
	return rb
}

func (rb *RequestBuilder) SetWriter(writer logger.Writer) *RequestBuilder {
	rb.request.Writer = writer
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
	auth := username + ":" + password
	token := base64.StdEncoding.EncodeToString([]byte(auth))
	rb.request.Headers.Set("Authorization", "Basic "+token)
	return rb
}

func (rb *RequestBuilder) WithBearer(token string) *RequestBuilder {
	rb.request.Headers.Set("Authorization", "Bearer "+token)
	return rb
}

func (rb *RequestBuilder) Execute() *Response {
	rb.setDefaultWriter()
	rb.setDefaultHeaders()
	rb.setQueryParams()

	httpClient := rb.client.httpClient
	httpClient.Header = rb.request.Headers

	if rb.request.Timeout > 0 {
		httpClient.SetTimeout(rb.request.Timeout * time.Millisecond)
	}
	httpClient.SetDebug(rb.request.DebugMode)
	rb.client.httpClient = httpClient

	return rb.request.doRequest(rb.client)
}

func (rb *RequestBuilder) setDefaultWriter() {
	if utils.IsEmpty(rb.request.Writer) {
		rb.SetWriter(&logger.DefaultWriter{ID: rb.request.ID})
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

// ExecuteWithRetry soon...
func (rb *RequestBuilder) ExecuteWithRetry(numberOfRetry int) *Response {
	return rb.request.doRequest(rb.client)
}
