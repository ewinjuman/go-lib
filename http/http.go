package http

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/ewinjuman/go-lib/v2/appContext"
	"net/http"
	"strings"
	"time"

	Error "github.com/ewinjuman/go-lib/v2/error"
	"github.com/ewinjuman/go-lib/v2/utils/convert"
	"github.com/go-resty/resty/v2"
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

type RequestConfig struct {
	Url         string
	Method      Method
	Headers     http.Header
	Payload     interface{}
	PathParams  map[string]string
	QueryParams map[string]string
	File        []MultipartData
}

type RestClient interface {
	DefaultHeader(username, password string) http.Header
	BasicAuth(username, password string) string
	Execute(appCtx *appContext.AppContext, method Method, url string, headers http.Header, payload interface{}, pathParams map[string]string, queryParam map[string]string, file []MultipartData) (body []byte, statusCode int, err error)
}

func New(options Options) RestClient {
	httpClient := resty.New()

	if options.SkipTLS {
		httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	httpClient.SetTimeout(options.Timeout * time.Second)
	httpClient.SetDebug(options.DebugMode)

	return &client{
		//options:        options,
		httpClient:     httpClient,
		circuitBreaker: NewCircuitBreaker(),
	}
}

type client struct {
	//options        Options
	httpClient     *resty.Client
	circuitBreaker *CircuitBreaker
}

func (c *client) DefaultHeader(username, password string) http.Header {
	headers := http.Header{}
	headers.Set("Authorization", "Basic "+c.BasicAuth(username, password))
	return headers
}

func (c *client) BasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (c *client) Execute(appCtx *appContext.AppContext, method Method, url string, headers http.Header, payload interface{}, pathParams map[string]string, queryParam map[string]string, file []MultipartData) (body []byte, statusCode int, err error) {

	//if c.circuitBreaker == nil {
	//	c.circuitBreaker = NewCircuitBreaker()
	//}

	//// Periksa apakah sirkuit diizinkan
	//if err := c.circuitBreaker.Allow(); err != nil {
	//	// Jika sirkuit terbuka, kembalikan error
	//	return nil, 0, err
	//}

	for key, value := range pathParams {
		url = strings.ReplaceAll(url, fmt.Sprintf(":%s", key), value)
	}
	request := c.httpClient.R()

	// Set header
	for h, val := range headers {
		request.Header[h] = val
	}
	if headers["Content-Type"] == nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("X-Request-ID", appCtx.RequestID)

	if queryParam != nil {
		request.SetQueryParams(queryParam)
	}

	// Set body
	switch request.Header.Get("Content-Type") {
	case "application/json":
		request.SetBody(payload)
		appCtx.Log().LogRequestHttp(appCtx.ToContext(), url, method.String(), request.Body, request.Header, request.QueryParam)
	case "application/x-www-form-urlencoded", "multipart/form-data":
		var formData map[string]string
		convert.ObjectToObject(payload, &formData)
		request.SetFormData(formData)
		appCtx.Log().LogRequestHttp(appCtx.ToContext(), url, method.String(), request.FormData, request.Header, request.QueryParam)
		if request.Header.Get("Content-Type") == "multipart/form-data" {
			for _, val := range file {
				request.SetFileReader(val.Key, val.Value, val.File)
			}
		}
	}

	// Execute rest
	var result *resty.Response
	var errExecute error
	switch method {
	case MethodPost:
		result, errExecute = request.Post(url)
	case MethodDelete:
		result, errExecute = request.Delete(url)
	case MethodGet:
		result, errExecute = request.Get(url)
	case MethodPut:
		result, errExecute = request.Put(url)
	case MethodOptions:
		result, errExecute = request.Options(url)
	case MethodPatch:
		result, errExecute = request.Patch(url)
	}
	responseTime := result.Time()
	// Check errExecute HTTP
	if errExecute != nil {
		//c.circuitBreaker.RecordFailure(errExecute)
		if result != nil {
			body = result.Body()
		}
		appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, statusCode, url, method.String(), body, errExecute)

		err = errExecute
		if Error.IsTimeout(errExecute) {
			err = Error.ErrDeadlineExceeded
		}
		return
	}

	//c.circuitBreaker.RecordSuccess()

	// Check,
	if result != nil {
		body = result.Body()
	}

	if result != nil && result.StatusCode() != 0 {
		statusCode = result.StatusCode()
	}

	switch result.Header().Get("Content-Type") {
	case "application/json":
		var result interface{}
		json.Unmarshal(body, &result)
		appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, statusCode, url, method.String(), result, nil)
	case "text/html":
		appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, statusCode, url, method.String(), string(body), nil)
	case "application/octet-stream":
		appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, statusCode, url, method.String(), "", nil)
	default:
		var result interface{}
		er := json.Unmarshal(body, &result)
		if er != nil {
			appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, statusCode, url, method.String(), string(body), nil)
		} else {
			appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, statusCode, url, method.String(), result, nil)
		}
	}

	if statusCode == http.StatusOK {
		return body, statusCode, nil
	}

	return body, statusCode, errExecute
}

type MultipartData struct {
	Key   string
	Value string
	File  *bytes.Reader
}
