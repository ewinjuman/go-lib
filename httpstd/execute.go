package httpstd

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	Error "github.com/ewinjuman/go-lib/v2/error"
	"github.com/ewinjuman/go-lib/v2/utils/convert"
)

var (
	jsonCheck            = regexp.MustCompile(`(?i:(application|text)/(.*json.*)(;|$))`)
	xmlCheck             = regexp.MustCompile(`(?i:(application|text)/(.*xml.*)(;|$))`)
	formContentType      = "application/x-www-form-urlencoded"
	multipartContentType = "multipart/form-data"

	bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}
)

func (r *Request) doRequest(client *reqClient) *Response {
	response := &Response{SuccessCodes: r.HTTPSuccessCode}

	var cb *CircuitBreaker
	if !r.DisableCircuitBreaker {
		cb = getCircuitBreaker(r.URL, r.CircuitBreakerConfig)
		if err := cb.Allow(); err != nil {
			response.Error = err
			return response
		}
	}

	rawURL := r.prepareURL()

	bodyReader, overrideContentType, err := r.buildBody()
	if err != nil {
		response.Error = err
		if cb != nil {
			cb.RecordFailure()
		}
		return response
	}

	ctx := r.Context
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, r.Method.String(), rawURL, bodyReader)
	if err != nil {
		response.Error = err
		if cb != nil {
			cb.RecordFailure()
		}
		return response
	}

	for key, vals := range r.Headers {
		for _, v := range vals {
			req.Header.Set(key, v)
		}
	}
	if overrideContentType != "" {
		req.Header.Set("Content-Type", overrideContentType)
	}

	if len(r.QueryParams) > 0 {
		q := req.URL.Query()
		for k, v := range r.QueryParams {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	if r.DebugMode {
		r.Writer.Print(r.Context, "http_request", r.Method.String(), rawURL, r.Body, r.Headers, r.QueryParams)
	}

	start := time.Now()
	resp, errDo := client.httpClient.Do(req)
	responseTime := time.Since(start)

	if errDo != nil {
		outErr := errDo
		if Error.IsTimeout(errDo) {
			outErr = Error.ErrDeadlineExceeded
		}
		response.Error = outErr
		if cb != nil {
			cb.RecordFailure()
		}
		if r.DebugMode {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), rawURL, 0, nil, http.Header{}, responseTime, outErr)
		}
		return response
	}
	defer resp.Body.Close()

	body, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		response.Error = errRead
		if cb != nil {
			cb.RecordFailure()
		}
		return response
	}

	response.StatusCode = resp.StatusCode
	response.Body = body

	if cb != nil {
		if resp.StatusCode >= 500 {
			cb.RecordFailure()
		} else {
			cb.RecordSuccess()
		}
	}

	if r.DebugMode {
		r.logResponse(response, resp.Header, rawURL, responseTime)
	}

	return response
}

func (r *Request) prepareURL() string {
	u := r.URL
	for key, value := range r.PathParams {
		u = strings.ReplaceAll(u, fmt.Sprintf(":%s", key), value)
	}
	return u
}

// buildBody encodes r.Body into an io.Reader.
// Returns the reader, an optional Content-Type override (for multipart), and any error.
func (r *Request) buildBody() (io.Reader, string, error) {
	contentType := r.Headers.Get("Content-Type")

	switch contentType {
	case formContentType:
		var m map[string]string
		convert.ObjectToObject(r.Body, &m)
		form := url.Values{}
		for k, v := range m {
			form.Set(k, v)
		}
		return strings.NewReader(form.Encode()), "", nil

	case multipartContentType:
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		var m map[string]string
		convert.ObjectToObject(r.Body, &m)
		for k, v := range m {
			if err := w.WriteField(k, v); err != nil {
				return nil, "", err
			}
		}
		for _, file := range r.File {
			part, err := w.CreateFormFile(file.Key, file.Value)
			if err != nil {
				return nil, "", err
			}
			if _, err = io.Copy(part, file.File); err != nil {
				return nil, "", err
			}
		}
		w.Close()
		return &buf, w.FormDataContentType(), nil

	default:
		if r.Body == nil {
			return nil, "", nil
		}
		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		if err := json.NewEncoder(buf).Encode(r.Body); err != nil {
			bufPool.Put(buf)
			return nil, "", err
		}
		// Copy to a separate reader so buf can be returned to the pool immediately.
		data := make([]byte, buf.Len())
		copy(data, buf.Bytes())
		bufPool.Put(buf)
		return bytes.NewReader(data), "", nil
	}
}

func (r *Request) logResponse(response *Response, header http.Header, rawURL string, responseTime time.Duration) {
	contentType := header.Get("Content-Type")
	var result interface{}

	switch {
	case xmlCheck.MatchString(contentType):
		if err := xml.Unmarshal(response.Body, &result); err != nil {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), rawURL, response.StatusCode, string(response.Body), header, responseTime, nil)
		} else {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), rawURL, response.StatusCode, result, header, responseTime, nil)
		}
	default:
		if err := json.Unmarshal(response.Body, &result); err != nil {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), rawURL, response.StatusCode, string(response.Body), header, responseTime, nil)
		} else {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), rawURL, response.StatusCode, result, header, responseTime, nil)
		}
	}
}
