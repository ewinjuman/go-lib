package http_v2

import (
	"encoding/json"
	"fmt"
	Error "github.com/ewinjuman/go-lib/v2/error"
	"github.com/ewinjuman/go-lib/v2/utils/convert"
	"github.com/go-resty/resty/v2"
	"strings"
	"time"
)

func (r *Request) DoRequest(client *ReqClient) (response *Response) {
	response = &Response{}
	url := r.URL
	for key, value := range r.PathParams {
		url = strings.ReplaceAll(url, fmt.Sprintf(":%s", key), value)
	}
	if r.Timeout > 0 {
		client.httpClient.SetTimeout(r.Timeout * time.Millisecond)
	}
	request := client.httpClient.R()

	// Set header
	for h, val := range r.Headers {
		request.Header[h] = val
	}
	if r.Headers["Content-Type"] == nil && r.Method != MethodGet {
		request.Header.Set("Content-Type", "application/json")
	}
	//request.Header.Set("X-Request-ID", appCtx.RequestID)

	if r.QueryParams != nil {
		request.SetQueryParams(r.QueryParams)
	}
	// Set body
	switch request.Header.Get("Content-Type") {
	case "application/json":
		request.SetBody(r.Body)
		r.Writer.Print("request_http", r.Method.String(), url, request.Body, request.Header, request.QueryParam)
		//appCtx.Log().LogRequestHttp(appCtx.ToContext(), url, r.Method.String(), request.Body, request.Header, request.QueryParam)
	case "application/x-www-form-urlencoded", "multipart/form-data":
		var formData map[string]string
		convert.ObjectToObject(r.Body, &formData)
		request.SetFormData(formData)
		r.Writer.Print("request_http", r.Method.String(), url, request.FormData, request.Header, request.QueryParam)
		//appCtx.Log().LogRequestHttp(appCtx.ToContext(), url, r.Method.String(), request.FormData, request.Header, request.QueryParam)
		if request.Header.Get("Content-Type") == "multipart/form-data" {
			for _, val := range r.File {
				request.SetFileReader(val.Key, val.Value, val.File)
			}
		}
	default:
		r.Writer.Print("request_http", r.Method.String(), url, request.Body, request.Header, request.QueryParam)

		//appCtx.Log().LogRequestHttp(appCtx.ToContext(), url, r.Method.String(), request.Body, request.Header, request.QueryParam)
	}

	// Execute rest
	var resultReq *resty.Response
	var errExecute error
	switch r.Method {
	case MethodPost:
		resultReq, errExecute = request.Post(url)
	case MethodDelete:
		resultReq, errExecute = request.Delete(url)
	case MethodGet:
		resultReq, errExecute = request.Get(url)
	case MethodPut:
		resultReq, errExecute = request.Put(url)
	case MethodOptions:
		resultReq, errExecute = request.Options(url)
	case MethodPatch:
		resultReq, errExecute = request.Patch(url)
	}
	responseTime := resultReq.Time()
	response.StatusCode = resultReq.StatusCode()
	// Check errExecute HTTP
	if errExecute != nil {
		//c.circuitBreaker.RecordFailure(errExecute)

		//appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), resultReq.Body(), errExecute)

		err := errExecute
		if Error.IsTimeout(errExecute) {
			err = Error.ErrDeadlineExceeded
		}
		response.Error = err
		if resultReq != nil {
			response.Body = resultReq.Body()
			r.Writer.Print("response_http", r.Method.String(), url, resultReq.Body(), resultReq.Header(), response.StatusCode, responseTime, err)
		} else {
			r.Writer.Print("response_http", r.Method.String(), url, nil, nil, response.StatusCode, responseTime, err)
		}
		return
	}

	//c.circuitBreaker.RecordSuccess()

	// Check,
	if resultReq != nil {
		response.Body = resultReq.Body()
	}

	if resultReq != nil && resultReq.StatusCode() != 0 {
		response.StatusCode = resultReq.StatusCode()
	}

	switch resultReq.Header().Get("Content-Type") {
	case "application/json":
		var result interface{}
		json.Unmarshal(response.Body, &result)
		r.Writer.Print("response_http", r.Method.String(), url, response.StatusCode, result, resultReq.Header(), responseTime)
		//appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), result, nil)
	case "text/html":
		r.Writer.Print("response_http", r.Method.String(), url, response.StatusCode, string(response.Body), resultReq.Header(), responseTime)
		//appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), string(response.Body), nil)
	case "application/octet-stream":
		r.Writer.Print("response_http", r.Method.String(), url, response.StatusCode, nil, resultReq.Header(), responseTime)
		//appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), "", nil)
	default:
		var result interface{}
		er := json.Unmarshal(response.Body, &result)
		if er != nil {
			r.Writer.Print("response_http", r.Method.String(), url, response.StatusCode, string(response.Body), resultReq.Header(), responseTime)
			//appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), string(response.Body), er)
		} else {
			r.Writer.Print("response_http", r.Method.String(), url, response.StatusCode, result, resultReq.Header(), responseTime)
			//appCtx.Log().LogResponseHttp(appCtx.ToContext(), responseTime, response.StatusCode, url, r.Method.String(), result, nil)
		}
	}

	return response
}
