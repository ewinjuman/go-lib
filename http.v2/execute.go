package http_v2

import (
	"encoding/json"
	"fmt"
	Error "github.com/ewinjuman/go-lib/v2/error"
	"github.com/ewinjuman/go-lib/v2/utils/convert"
	"github.com/go-resty/resty/v2"
	"log"
	"strings"
	"time"
)

func (r *Request) DoRequest(httpClient *resty.Client) (response *Response) {
	appCtx := r.appContext
	response = &Response{}
	url := r.URL
	for key, value := range r.PathParams {
		url = strings.ReplaceAll(url, fmt.Sprintf(":%s", key), value)
	}
	if r.Timeout > 0 {
		httpClient.SetTimeout(r.Timeout * time.Millisecond)
	}
	request := httpClient.R()

	// Set header
	for h, val := range r.Headers {
		request.Header[h] = val
	}
	if r.Headers["Content-Type"] == nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("X-Request-ID", "appCtx.RequestID")

	if r.QueryParams != nil {
		request.SetQueryParams(r.QueryParams)
	}

	// Set body
	switch request.Header.Get("Content-Type") {
	case "application/json":
		request.SetBody(r.Body)
		appCtx.Log().LogRequestHttp(appCtx.ToContext(), url, r.Method.String(), request.Body, request.Header, request.QueryParam)
	case "application/x-www-form-urlencoded", "multipart/form-data":
		var formData map[string]string
		convert.ObjectToObject(r.Body, &formData)
		request.SetFormData(formData)
		appCtx.Log().LogRequestHttp(appCtx.ToContext(), url, r.Method.String(), request.FormData, request.Header, request.QueryParam)
		if request.Header.Get("Content-Type") == "multipart/form-data" {
			for _, val := range r.File {
				request.SetFileReader(val.Key, val.Value, val.File)
			}
		}
	}

	// Execute rest
	var result *resty.Response
	var errExecute error
	switch r.Method {
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
	response.StatusCode = result.StatusCode()
	// Check errExecute HTTP
	if errExecute != nil {
		//c.circuitBreaker.RecordFailure(errExecute)
		if result != nil {
			response.Body = result.Body()
		}
		appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), result.Body(), errExecute)

		err := errExecute
		if Error.IsTimeout(errExecute) {
			err = Error.ErrDeadlineExceeded
		}
		response.Error = err
		return
	}

	//c.circuitBreaker.RecordSuccess()

	// Check,
	if result != nil {
		response.Body = result.Body()
	}

	if result != nil && result.StatusCode() != 0 {
		response.StatusCode = result.StatusCode()
	}

	switch result.Header().Get("Content-Type") {
	case "application/json":
		var result interface{}
		json.Unmarshal(response.Body, &result)
		appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), result, nil)
	case "text/html":
		appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), string(response.Body), nil)
	case "application/octet-stream":
		appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), "", nil)
	default:
		var result interface{}
		er := json.Unmarshal(response.Body, &result)
		if er != nil {
			appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), string(response.Body), nil)
		} else {
			appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), result, nil)
		}
	}

	return response
}

//func (d *RequestBuilder) Do() *Response {
//
//}

func (r *Response) Consume(v interface{}) error {
	if r.Error != nil {
		return r.Error
	}

	if r.StatusCode < 200 || r.StatusCode > 299 {
		log.Println("statusCode", r.StatusCode)
		log.Println("body", r.Body)
		log.Println("Error when make request")

		body := ""
		if r.Body != nil {
			body = string(r.Body)
		}

		return fmt.Errorf("Response return status not OK, with status code %d, and body %s",
			r.StatusCode,
			body,
		)
	}

	if r.Body == nil {
		return ErrEmptyResponseBody
	}

	if err := json.Unmarshal(r.Body, &v); err != nil {
		return fmt.Errorf("failed copying response body to interface, cause %s, responseBody %s",
			err.Error(),
			string(r.Body),
		)
	}

	return nil
}
