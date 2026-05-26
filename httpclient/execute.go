// httpclient/execute.go
package httpclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	Error "github.com/ewinjuman/go-lib/v2/apperror"
	"github.com/ewinjuman/go-lib/v2/logger"
	"github.com/ewinjuman/go-lib/v2/utils/convert"
	"github.com/google/uuid"
)

var (
	jsonCheck = regexp.MustCompile(`(?i:(application|text)/(.*json.*)(;|$))`)
	xmlCheck  = regexp.MustCompile(`(?i:(application|text)/(.*xml.*)(;|$))`)
	bufPool   = sync.Pool{New: func() any { return new(bytes.Buffer) }}
)

// buildURL joins the client base URL with rb.path and replaces :param placeholders.
func (rb *RequestBuilder) buildURL() string {
	p := rb.path
	for key, val := range rb.pathParams {
		p = strings.ReplaceAll(p, ":"+key, val)
	}
	if rb.client.baseURL != "" &&
		!strings.HasPrefix(p, "http://") &&
		!strings.HasPrefix(p, "https://") {
		return strings.TrimRight(rb.client.baseURL, "/") + "/" + strings.TrimLeft(p, "/")
	}
	return p
}

// buildBody encodes rb.body into an io.Reader.
// Returns the reader, an optional Content-Type override, and any error.
func (rb *RequestBuilder) buildBody() (io.Reader, string, error) {
	switch rb.bodyMode {
	case bodyModeNone:
		return nil, "", nil

	case bodyModeJSON:
		if rb.body == nil {
			return nil, "", nil
		}
		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		if err := json.NewEncoder(buf).Encode(rb.body); err != nil {
			bufPool.Put(buf)
			return nil, "", err
		}
		data := make([]byte, buf.Len())
		copy(data, buf.Bytes())
		bufPool.Put(buf)
		return bytes.NewReader(data), "", nil

	case bodyModeForm:
		var m map[string]string
		convert.ObjectToObject(rb.body, &m)
		form := url.Values{}
		for k, v := range m {
			form.Set(k, v)
		}
		return strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", nil

	case bodyModeMultipart:
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for _, part := range rb.multipartParts {
			if part.isFile {
				var r io.Reader
				if part.reader != nil {
					r = part.reader
				} else {
					f, err := os.Open(part.value)
					if err != nil {
						return nil, "", fmt.Errorf("open multipart file %q: %w", part.value, err)
					}
					defer f.Close()
					r = f
				}
				fw, err := w.CreateFormFile(part.field, part.filename)
				if err != nil {
					return nil, "", err
				}
				if _, err = io.Copy(fw, r); err != nil {
					return nil, "", err
				}
			} else {
				if err := w.WriteField(part.field, part.value); err != nil {
					return nil, "", err
				}
			}
		}
		if err := w.Close(); err != nil {
			return nil, "", fmt.Errorf("finalize multipart body: %w", err)
		}
		return &buf, w.FormDataContentType(), nil

	case bodyModeRaw:
		return bytes.NewReader(rb.rawBody), rb.rawContentType, nil
	}
	return nil, "", nil
}

// applyDefaults fills in headers and auth that were not set per-request.
func (rb *RequestBuilder) applyDefaults() {
	if rb.requestID != "" {
		rb.headers.Set("X-Request-ID", rb.requestID)
	} else if rb.headers.Get("X-Request-ID") == "" {
		rb.headers.Set("X-Request-ID", uuid.New().String())
	}
	if rb.headers.Get("Content-Type") == "" && rb.method != MethodGet && rb.bodyMode == bodyModeJSON {
		rb.headers.Set("Content-Type", "application/json")
	}
	if rb.headers.Get("Authorization") == "" {
		if rb.client.defaultBearer != nil {
			rb.headers.Set("Authorization", "Bearer "+rb.client.defaultBearer())
		} else if rb.client.defaultBasicUser != "" {
			token := base64.StdEncoding.EncodeToString(
				[]byte(rb.client.defaultBasicUser + ":" + rb.client.defaultBasicPass),
			)
			rb.headers.Set("Authorization", "Basic "+token)
		}
	}
	if rb.debug && rb.writer == nil {
		rb.writer = &logger.DefaultWriter{ID: rb.requestID}
	}
}

func (rb *RequestBuilder) logResponse(ctx context.Context, response *Response, header http.Header, rawURL string, responseTime time.Duration) {
	contentType := header.Get("Content-Type")
	var result any
	switch {
	case xmlCheck.MatchString(contentType):
		if err := xml.Unmarshal(response.Body, &result); err != nil {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, response.StatusCode, string(response.Body), header, responseTime, nil)
		} else {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, response.StatusCode, result, header, responseTime, nil)
		}
	default:
		if err := json.Unmarshal(response.Body, &result); err != nil {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, response.StatusCode, string(response.Body), header, responseTime, nil)
		} else {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, response.StatusCode, result, header, responseTime, nil)
		}
	}
}

// Execute sends the HTTP request through the composed middleware stack
// and returns a Response. It is the terminal step of the fluent builder.
func (rb *RequestBuilder) Execute() *Response {
	rb.applyDefaults()
	response := &Response{SuccessCodes: rb.successCodes}

	bodyReader, overrideContentType, err := rb.buildBody()
	if err != nil {
		response.Error = err
		return response
	}

	rawURL := rb.buildURL()
	ctx := rb.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, rb.method.String(), rawURL, bodyReader)
	if err != nil {
		response.Error = err
		return response
	}

	for k, vals := range rb.headers {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}
	if overrideContentType != "" {
		req.Header.Set("Content-Type", overrideContentType)
	}

	if len(rb.queryParams) > 0 {
		q := req.URL.Query()
		for k, v := range rb.queryParams {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	for _, c := range rb.cookies {
		req.AddCookie(c)
	}

	timeout := rb.timeout
	if timeout == 0 {
		timeout = rb.client.defaultTimeout
	}

	hc := rb.client.buildHTTPClient(rb.skipTLS, timeout)
	doer := Apply(hc, rb.client.middlewares, rb.skipMiddleware)

	if rb.debug && rb.writer != nil {
		rb.writer.Print(ctx, "http_request", rb.method.String(), rawURL, rb.body, rb.headers, rb.queryParams)
	}

	start := time.Now()
	resp, errDo := doer.Do(req)
	responseTime := time.Since(start)

	if errDo != nil {
		outErr := errDo
		if Error.IsTimeout(errDo) {
			outErr = Error.ErrDeadlineExceeded
		}
		response.Error = outErr
		if rb.debug && rb.writer != nil {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, 0, nil, http.Header{}, responseTime, outErr)
		}
		return response
	}
	defer resp.Body.Close()

	response.StatusCode = resp.StatusCode
	response.Headers = resp.Header
	response.raw = resp

	if rb.output != nil {
		_, copyErr := io.Copy(rb.output, resp.Body)
		if rb.debug && rb.writer != nil {
			rb.writer.Print(ctx, "http_response", rb.method.String(), rawURL, response.StatusCode, "[streamed]", resp.Header, responseTime, copyErr)
		}
		if copyErr != nil {
			response.Error = copyErr
		}
		return response
	}

	bodyBytes, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		response.Error = errRead
		return response
	}
	response.Body = bodyBytes

	if rb.debug && rb.writer != nil {
		rb.logResponse(ctx, response, resp.Header, rawURL, responseTime)
	}

	return response
}
