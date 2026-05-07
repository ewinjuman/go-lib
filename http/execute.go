package http

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"time"

	Error "github.com/ewinjuman/go-lib/v2/error"
	"github.com/ewinjuman/go-lib/v2/utils/convert"
	"github.com/go-resty/resty/v2"
)

var (
	plainTextType        = "text/plain; charset=utf-8"
	jsonContentType      = "application/json"
	formContentType      = "application/x-www-form-urlencoded"
	multipartContentType = "multipart/form-data"

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
	response = &Response{SuccessCodes: r.HTTPSuccessCode}

	var cb *CircuitBreaker
	if !r.DisableCircuitBreaker {
		cb = getCircuitBreaker(r.URL, r.CircuitBreakerConfig)
		if err := cb.Allow(); err != nil {
			response.Error = err
			return response
		}
	}

	request := client.httpClient.R()
	url := r.prepareURL()

	r.prepareRequestBody(request, url)

	resultRequest, errExecute := r.executeRequest(request, url)
	responseTime := r.calculateResponseTime(resultRequest)

	if errExecute != nil {
		if cb != nil {
			cb.RecordFailure()
		}
		return r.handleError(response, resultRequest, errExecute, url, responseTime)
	}

	// 5xx indicates server/infrastructure issues; 4xx means server is up but rejected request
	if cb != nil {
		if resultRequest.StatusCode() >= 500 {
			cb.RecordFailure()
		} else {
			cb.RecordSuccess()
		}
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
	case jsonContentType:
		request.SetBody(r.Body)
	case formContentType, multipartContentType:
		var formData map[string]string
		convert.ObjectToObject(r.Body, &formData)
		request.SetFormData(formData)
		if contentType == multipartContentType {
			for _, file := range r.File {
				request.SetFileReader(file.Key, file.Value, file.File)
			}
		}
	}
	if r.DebugMode {
		r.Writer.Print(r.Context, "http_request", r.Method.String(), url, request.Body, r.Headers, r.QueryParams)
	}
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
	if r.DebugMode {
		r.Writer.Print(r.Context, "http_response", r.Method.String(), url, response.StatusCode, response.Body, resultRequest.Header(), responseTime, err)
	}
	return response
}

// processResponse unmarshal and processes the response body on success
func (r *Request) processResponse(response *Response, resultRequest *resty.Response, url string, responseTime time.Duration) {
	response.Body = resultRequest.Body()
	response.StatusCode = resultRequest.StatusCode()
	if !r.DebugMode {
		return
	}
	var result interface{}
	contentType := resultRequest.Header().Get("Content-Type")
	switch contentType {
	case "application/xml; charset=utf-8":
		if err := xml.Unmarshal(response.Body, &result); err != nil {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), url, response.StatusCode, string(response.Body), resultRequest.Header(), responseTime, nil)
		} else {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), url, response.StatusCode, result, resultRequest.Header(), responseTime, nil)
		}
	default:
		if err := json.Unmarshal(response.Body, &result); err != nil {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), url, response.StatusCode, string(response.Body), resultRequest.Header(), responseTime, nil)
		} else {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), url, response.StatusCode, result, resultRequest.Header(), responseTime, nil)
		}
	}

}
