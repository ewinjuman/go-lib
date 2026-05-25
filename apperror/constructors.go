package apperror

import (
  "errors"
  "net/http"
)

// ── Named sentinel errors ─────────────────────────────────────────────────────
//
// Use these as comparison targets with errors.Is:
//
//	if errors.Is(err, apperror.ErrNotFound) { ... }
//
// Create domain-specific errors by starting from the sentinel:
//
//	return apperror.ErrNotFound.WithCause(sql.ErrNoRows)
//
// Or supply a custom message via NewError:
//
//	return apperror.NewError(http.StatusNotFound, apperror.FailedStatus, "user not found")

var (
  ErrBadRequest          = newSentinel(http.StatusBadRequest, FailedStatus)
  ErrUnauthorized        = newSentinel(http.StatusUnauthorized, FailedStatus)
  ErrForbidden           = newSentinel(http.StatusForbidden, FailedStatus)
  ErrNotFound            = newSentinel(http.StatusNotFound, FailedStatus)
  ErrConflict            = newSentinel(http.StatusConflict, FailedStatus)
  ErrUnprocessableEntity = newSentinel(http.StatusUnprocessableEntity, FailedStatus)
  ErrTooManyRequests     = newSentinel(http.StatusTooManyRequests, FailedStatus)
  ErrInternalError       = newSentinel(http.StatusInternalServerError, FailedStatus)
  ErrServiceUnavailable  = newSentinel(http.StatusServiceUnavailable, FailedStatus)
)

// newSentinel builds a minimal ApplicationError used as a sentinel value.
// The message is the standard HTTP status text for the code.
func newSentinel(code int, stat string) *ApplicationError {
  return &ApplicationError{
    ErrorCode: code,
    Status:    stat,
    Message:   StatusMessage(code),
  }
}

// ── Named constructors ────────────────────────────────────────────────────────
//
// Each constructor accepts an optional message. When omitted the standard HTTP
// status text is used. They return *ApplicationError (not error) so callers can
// chain .WithCause without a type assertion.

// BadRequest returns a 400 ApplicationError.
func BadRequest(message ...string) *ApplicationError {
  return build(http.StatusBadRequest, FailedStatus, message...)
}

// Unauthorized returns a 401 ApplicationError.
func Unauthorized(message ...string) *ApplicationError {
  return build(http.StatusUnauthorized, FailedStatus, message...)
}

// Forbidden returns a 403 ApplicationError.
func Forbidden(message ...string) *ApplicationError {
  return build(http.StatusForbidden, FailedStatus, message...)
}

// NotFound returns a 404 ApplicationError.
func NotFound(message ...string) *ApplicationError {
  return build(http.StatusNotFound, FailedStatus, message...)
}

// Conflict returns a 409 ApplicationError.
func Conflict(message ...string) *ApplicationError {
  return build(http.StatusConflict, FailedStatus, message...)
}

// UnprocessableEntity returns a 422 ApplicationError.
func UnprocessableEntity(message ...string) *ApplicationError {
  return build(http.StatusUnprocessableEntity, FailedStatus, message...)
}

// TooManyRequests returns a 429 ApplicationError.
func TooManyRequests(message ...string) *ApplicationError {
  return build(http.StatusTooManyRequests, FailedStatus, message...)
}

// InternalError returns a 500 ApplicationError.
func InternalError(message ...string) *ApplicationError {
  return build(http.StatusInternalServerError, FailedStatus, message...)
}

// ServiceUnavailable returns a 503 ApplicationError.
func ServiceUnavailable(message ...string) *ApplicationError {
  return build(http.StatusServiceUnavailable, FailedStatus, message...)
}

// build is the shared factory used by all named constructors.
func build(code int, stat string, message ...string) *ApplicationError {
  msg := StatusMessage(code)
  if len(message) > 0 {
    msg = message[0]
  }
  return &ApplicationError{
    ErrorCode: code,
    Status:    stat,
    Message:   msg,
  }
}

// ── Named predicates ──────────────────────────────────────────────────────────
//
// Each predicate uses errors.Is so it works with wrapped errors.

// IsNotFound reports whether err is (or wraps) a 404 ApplicationError.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsBadRequest reports whether err is (or wraps) a 400 ApplicationError.
func IsBadRequest(err error) bool { return errors.Is(err, ErrBadRequest) }

// IsUnauthorized reports whether err is (or wraps) a 401 ApplicationError.
func IsUnauthorized(err error) bool { return errors.Is(err, ErrUnauthorized) }

// IsForbidden reports whether err is (or wraps) a 403 ApplicationError.
func IsForbidden(err error) bool { return errors.Is(err, ErrForbidden) }

// IsConflict reports whether err is (or wraps) a 409 ApplicationError.
func IsConflict(err error) bool { return errors.Is(err, ErrConflict) }

// IsUnprocessable reports whether err is (or wraps) a 422 ApplicationError.
func IsUnprocessable(err error) bool { return errors.Is(err, ErrUnprocessableEntity) }

// IsTooManyRequests reports whether err is (or wraps) a 429 ApplicationError.
func IsTooManyRequests(err error) bool { return errors.Is(err, ErrTooManyRequests) }

// IsInternalError reports whether err is (or wraps) a 500 ApplicationError.
func IsInternalError(err error) bool { return errors.Is(err, ErrInternalError) }

// IsServiceUnavailable reports whether err is (or wraps) a 503 ApplicationError.
func IsServiceUnavailable(err error) bool { return errors.Is(err, ErrServiceUnavailable) }
