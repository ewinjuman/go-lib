package http_v2

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/ewinjuman/go-lib/v2/logger"
	"github.com/ewinjuman/go-lib/v2/utils"

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
		client  *ReqClient
		//requestManager RequestManager
		//requestRetry   RequestRetry
	}
)

func Do(ctx context.Context, method Method, host, path string) *RequestBuilder {

	url := host + path
	return &RequestBuilder{
		request: Request{
			//AppContext: appContext,
			URL:     url,
			Method:  method,
			Headers: http.Header{},
		},
		client: httpclient(),
		//requestRetry:   &RequestRetryWhenTimeout{},
	}
}

func Post(ctx context.Context, host, endpoint string) *RequestBuilder {
	return Do(ctx, MethodPost, host, endpoint)
}

func Get(ctx context.Context, host, endpoint string) *RequestBuilder {
	return Do(ctx, MethodGet, host, endpoint)
}

func Put(ctx context.Context, host, endpoint string) *RequestBuilder {
	return Do(ctx, MethodPut, host, endpoint)
}

func Delete(ctx context.Context, host, endpoint string) *RequestBuilder {
	return Do(ctx, MethodDelete, host, endpoint)
}

func Patch(ctx context.Context, host, endpoint string) *RequestBuilder {
	return Do(ctx, MethodPatch, host, endpoint)
}

func Options(ctx context.Context, host, endpoint string) *RequestBuilder {
	return Do(ctx, MethodOptions, host, endpoint)
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

func (rb *RequestBuilder) WithPathHeaders(headers map[string]string) *RequestBuilder {
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
	if utils.IsEmpty(rb.request.Writer) {
		log, _ := logger.New(logger.DefaultOptions())
		rb.SetWriter(log)
	}
	return rb.request.DoRequest(rb.request.Writer, rb.client)
}

// ExecuteWithRetry soon...
func (rb *RequestBuilder) ExecuteWithRetry(numberOfRetry int) *Response {
	return rb.request.DoRequest(rb.request.Writer, rb.client)
}
