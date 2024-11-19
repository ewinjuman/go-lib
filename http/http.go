package http

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	Error "github.com/ewinjuman/go-lib/error"
	"github.com/ewinjuman/go-lib/helper/convert"
	Session "github.com/ewinjuman/go-lib/session"
	"github.com/go-resty/resty/v2"
)

type Method string

type RequestConfig struct {
	Method     Method
	Host       string
	Path       string
	Header     http.Header
	Payload    interface{}
	QueryParam map[string]string
	File       []MultipartData
}

type RestClient interface {
	DefaultHeader(username, password string) http.Header
	BasicAuth(username, password string) string
	Execute(session *Session.Session, host string, path string, method string, headers http.Header, payload interface{}, queryParam map[string]string, file []MultipartData) (body []byte, statusCode int, err error)
	//Request(request RequestConfig) (body []byte, statusCode int, err error)
	//Execute(session *Session.Session, host string, path string, method string, headers http.Header, payload interface{}, PathParams map[string]string, queryParam map[string]string, file []MultipartData) (body []byte, statusCode int, err error)
}

const (
	MethodPost   Method = "POST"
	MethodGet    Method = "GET"
	MethodPut    Method = "PUT"
	MethodDelete Method = "DELETE"
	MethodPatch  Method = "PATCH"
	MethodOption Method = "OPTION"
)

func New(options Options) RestClient {
	httpClient := resty.New()

	if options.SkipTLS {
		httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	httpClient.SetTimeout(options.Timeout * time.Second)
	httpClient.SetDebug(options.DebugMode)

	return &client{
		options:    options,
		httpClient: httpClient,
	}
}

type client struct {
	options    Options
	httpClient *resty.Client
	session    *Session.Session
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

func (c *client) Execute(session *Session.Session, host string, path string, method string, headers http.Header, payload interface{}, queryParam map[string]string, file []MultipartData) (body []byte, statusCode int, err error) {
	url := host + path
	request := c.httpClient.R()

	// Set header
	for h, val := range headers {
		request.Header[h] = val
	}
	if headers["Content-Type"] == nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("X-Request-ID", session.ThreadID)

	if queryParam != nil {
		request.SetQueryParams(queryParam)
	}

	// Set body
	switch request.Header.Get("Content-Type") {
	case "application/json":
		request.SetBody(payload)
		session.LogRequestHttp(url, method, request.Body, request.Header, request.QueryParam)
	case "application/x-www-form-urlencoded", "multipart/form-data":
		var formData map[string]string
		convert.ObjectToObject(payload, &formData)
		request.SetFormData(formData)
		session.LogRequestHttp(url, method, request.FormData, request.Header, request.QueryParam)
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
	case http.MethodPost:
		result, errExecute = request.Post(url)
	case http.MethodDelete:
		result, errExecute = request.Delete(url)
	case http.MethodGet:
		result, errExecute = request.Get(url)
	case http.MethodPut:
		result, errExecute = request.Put(url)
	case http.MethodOptions:
		result, errExecute = request.Options(url)
	case http.MethodPatch:
		result, errExecute = request.Patch(url)
	}

	responseTime := result.Time()
	// Check errExecute HTTP
	if errExecute != nil {
		if result != nil {
			body = result.Body()
		}
		session.LogResponseHttp(responseTime, statusCode, url, method, body, errExecute.Error())

		err = errExecute
		if Error.IsTimeout(errExecute) {
			err = Error.ErrDeadlineExceeded
		}
		return
	}

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
		session.LogResponseHttp(responseTime, statusCode, url, method, result)
	case "text/html":
		session.LogResponseHttp(responseTime, statusCode, url, method, string(body))
	case "application/octet-stream":
		session.LogResponseHttp(responseTime, statusCode, url, method, "")
	default:
		var result interface{}
		er := json.Unmarshal(body, &result)
		if er != nil {
			session.LogResponseHttp(responseTime, statusCode, url, method, string(body))
		} else {
			session.LogResponseHttp(responseTime, statusCode, url, method, result)
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
