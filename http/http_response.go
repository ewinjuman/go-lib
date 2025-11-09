package http

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Response struct {
	StatusCode int
	Body       []byte
	Error      error
}

var (
	ErrEmptyResponseBody = errors.New("Response body is empty")
)

// Consume : v is pointer Object
func (r *Response) Consume(v interface{}) error {
	if r.Error != nil {
		return r.Error
	}

	if r.StatusCode < 200 || r.StatusCode >= 300 {
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

func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}
func (r *Response) IsError() (bool, error) {
	if r.Error != nil {
		return true, r.Error
	}
	return r.StatusCode < 200 || r.StatusCode >= 300, r.Error
}

func (r *Response) HttpCode() int {
	return r.StatusCode
}
