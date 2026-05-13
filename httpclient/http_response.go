package httpclient

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Response struct {
	StatusCode   int
	Body         []byte
	Error        error
	SuccessCodes []int
}

var ErrEmptyResponseBody = errors.New("Response body is empty")

// isSuccessStatus checks against SuccessCodes if set, otherwise falls back to 2xx.
func (r *Response) isSuccessStatus() bool {
	if len(r.SuccessCodes) > 0 {
		for _, code := range r.SuccessCodes {
			if r.StatusCode == code {
				return true
			}
		}
		return false
	}
	return r.StatusCode >= 200 && r.StatusCode < 300
}

func (r *Response) statusError() error {
	body := ""
	if r.Body != nil {
		body = string(r.Body)
	}
	return fmt.Errorf("response status not OK, status code %d, body: %s", r.StatusCode, body)
}

// Consume unmarshals the response body into v. Returns an error if the status is not successful.
func (r *Response) Consume(v interface{}) error {
	if r.Error != nil {
		return r.Error
	}
	if !r.isSuccessStatus() {
		return r.statusError()
	}
	if r.Body == nil {
		return ErrEmptyResponseBody
	}
	if err := json.Unmarshal(r.Body, &v); err != nil {
		return fmt.Errorf("failed copying response body to interface, cause %s, responseBody %s", err.Error(), string(r.Body))
	}
	return nil
}

func (r *Response) IsSuccess() bool {
	return r.Error == nil && r.isSuccessStatus()
}

func (r *Response) IsError() (bool, error) {
	if r.Error != nil {
		return true, r.Error
	}
	if !r.isSuccessStatus() {
		return true, r.statusError()
	}
	return false, nil
}

func (r *Response) HttpCode() int {
	return r.StatusCode
}
