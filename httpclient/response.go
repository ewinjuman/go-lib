// httpclient/response.go
package httpclient

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"os"
)

// ErrEmptyResponseBody is returned by Consume/SaveToFile when Body is nil
// (e.g. after streaming via WithOutput).
var ErrEmptyResponseBody = errors.New("response body is empty")

// Response holds the result of an HTTP request.
type Response struct {
	StatusCode   int
	Body         []byte      // nil when WithOutput was used (body streamed directly)
	Headers      http.Header // response headers — always populated on non-streaming responses
	Error        error
	SuccessCodes []int
	raw          *http.Response // underlying response; body already closed after Execute
}

func (r *Response) isSuccessStatus() bool {
	for _, c := range r.SuccessCodes {
		if r.StatusCode == c {
			return true
		}
	}
	return false
}

func (r *Response) statusError() error {
	body := ""
	if r.Body != nil {
		body = string(r.Body)
	}
	return fmt.Errorf("response status not OK: %d, body: %s", r.StatusCode, body)
}

// IsSuccess returns true when there is no transport error and status is in SuccessCodes.
func (r *Response) IsSuccess() bool {
	return r.Error == nil && r.isSuccessStatus()
}

// IsError returns (true, err) when there is a transport error or status not in SuccessCodes.
func (r *Response) IsError() (bool, error) {
	if r.Error != nil {
		return true, r.Error
	}
	if !r.isSuccessStatus() {
		return true, r.statusError()
	}
	return false, nil
}

// HttpCode returns the HTTP status code.
func (r *Response) HttpCode() int { return r.StatusCode }

// Consume JSON-unmarshals Body into v. Returns an error if transport failed,
// status is not in SuccessCodes, or Body is nil.
func (r *Response) Consume(v any) error {
	if r.Error != nil {
		return r.Error
	}
	if !r.isSuccessStatus() {
		return r.statusError()
	}
	if r.Body == nil {
		return ErrEmptyResponseBody
	}
	if err := json.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w (body: %s)", err, string(r.Body))
	}
	return nil
}

// ConsumeXML XML-unmarshals Body into v. Same preconditions as Consume.
func (r *Response) ConsumeXML(v any) error {
	if r.Error != nil {
		return r.Error
	}
	if !r.isSuccessStatus() {
		return r.statusError()
	}
	if r.Body == nil {
		return ErrEmptyResponseBody
	}
	if err := xml.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("failed to unmarshal XML response body: %w", err)
	}
	return nil
}

// ConsumeText returns Body as a string. Same preconditions as Consume.
func (r *Response) ConsumeText() (string, error) {
	if r.Error != nil {
		return "", r.Error
	}
	if !r.isSuccessStatus() {
		return "", r.statusError()
	}
	if r.Body == nil {
		return "", ErrEmptyResponseBody
	}
	return string(r.Body), nil
}

// SaveToFile writes Body to the file at path with permission 0644.
// Returns ErrEmptyResponseBody when Body is nil (e.g. after WithOutput streaming).
func (r *Response) SaveToFile(path string) error {
	if r.Error != nil {
		return r.Error
	}
	if !r.isSuccessStatus() {
		return r.statusError()
	}
	if r.Body == nil {
		return ErrEmptyResponseBody
	}
	return os.WriteFile(path, r.Body, 0644)
}

// Raw returns the underlying *http.Response. The Body is already read and closed
// by Execute — do not re-read it. Useful for Trailer, TLS state, etc.
func (r *Response) Raw() *http.Response { return r.raw }
