package http

import (
	"encoding/json"
	"fmt"
	Error "github.com/ewinjuman/go-lib/v2/error"
	"github.com/ewinjuman/go-lib/v2/utils/convert"
	"github.com/go-resty/resty/v2"
	"regexp"
	"strings"
	"time"
)

var (
	plainTextType   = "text/plain; charset=utf-8"
	jsonContentType = "application/json"
	formContentType = "application/x-www-form-urlencoded"

	jsonCheck = regexp.MustCompile(`(?i:(application|text)/(.*json.*)(;|$))`)
	xmlCheck  = regexp.MustCompile(`(?i:(application|text)/(.*xml.*)(;|$))`)
)

func (r *Request) calculateResponseTime(resultRequest *resty.Response) time.Duration {
	if resultRequest == nil {
		return 0 * time.Second
	}
	return resultRequest.Time()
}

func (r *Request) doRequest(client *reqClient) (response *Response) {
	response = &Response{}
	request := client.httpClient.R()
	url := r.prepareURL()

	r.prepareRequestBody(request, url)

	resultRequest, errExecute := r.executeRequest(request, url)
	responseTime := r.calculateResponseTime(resultRequest)

	if errExecute != nil {
		return r.handleError(response, resultRequest, errExecute, url, responseTime)
	}

	r.processResponse(response, resultRequest, url, responseTime)
	return response
}

// prepareURL replaces path parameters in the URL
func (r *Request) prepareURL() string {
	url := r.URL
	for key, value := range r.PathParams {
		url = strings.ReplaceAll(url, fmt.Sprintf(":%s", key), value)
	}
	return url
}

// prepareRequestBody sets the body, form data, and file data to the request
func (r *Request) prepareRequestBody(request *resty.Request, url string) {
	contentType := r.Headers.Get("Content-Type")
	switch contentType {
	case "application/json":
		request.SetBody(r.Body)
	case "application/x-www-form-urlencoded", "multipart/form-data":
		var formData map[string]string
		convert.ObjectToObject(r.Body, &formData)
		request.SetFormData(formData)
		if contentType == "multipart/form-data" {
			for _, file := range r.File {
				request.SetFileReader(file.Key, file.Value, file.File)
			}
		}
	}

	r.Writer.Print("http_request", r.Method.String(), url, request.Body, r.Headers, r.QueryParams)
}

// executeRequest sends the prepared HTTP request based on the method
func (r *Request) executeRequest(request *resty.Request, url string) (*resty.Response, error) {
	switch r.Method {
	case MethodPost:
		return request.Post(url)
	case MethodDelete:
		return request.Delete(url)
	case MethodGet:
		return request.Get(url)
	case MethodPut:
		return request.Put(url)
	case MethodOptions:
		return request.Options(url)
	case MethodPatch:
		return request.Patch(url)
	default:
		return nil, fmt.Errorf("unsupported HTTP method")
	}
}

// handleError processes error scenarios from the request execution
func (r *Request) handleError(response *Response, resultRequest *resty.Response, errExecute error, url string, responseTime time.Duration) *Response {
	err := errExecute
	if Error.IsTimeout(errExecute) {
		err = Error.ErrDeadlineExceeded
	}
	response.Error = err
	if resultRequest != nil {
		response.Body = resultRequest.Body()
	}
	r.Writer.Print("http_response", r.Method.String(), url, response.StatusCode, response.Body, resultRequest.Header(), responseTime, err)
	return response
}

// processResponse unmarshals and processes the response body on success
func (r *Request) processResponse(response *Response, resultRequest *resty.Response, url string, responseTime time.Duration) {
	response.Body = resultRequest.Body()
	response.StatusCode = resultRequest.StatusCode()
	var result interface{}
	if err := json.Unmarshal(response.Body, &result); err != nil {
		r.Writer.Print("http_response", r.Method.String(), url, response.StatusCode, string(response.Body), resultRequest.Header(), responseTime, nil)
	} else {
		r.Writer.Print("http_response", r.Method.String(), url, response.StatusCode, result, resultRequest.Header(), responseTime, nil)
	}
}
