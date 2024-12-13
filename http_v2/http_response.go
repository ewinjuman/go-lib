package http_v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

	if r.StatusCode < 200 || r.StatusCode > 299 {
		log.Println("statusCode", r.StatusCode)
		log.Println("body", r.Body)
		log.Println("Error when make Request")

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
