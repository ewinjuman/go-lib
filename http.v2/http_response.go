package http_v2

import (
	"errors"
)

type Response struct {
	StatusCode int
	Body       []byte
	Error      error
}

var (
	ErrEmptyResponseBody = errors.New("Response body is empty")
)
