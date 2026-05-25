package apperror

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil error returns 200", nil, 200},
		{"ApplicationError returns its code", NewError(451, "FAILED", "set error pending"), 451},
		{"plain error returns 500", errors.New("set error"), 500},
		{"wrapped ApplicationError returns inner code", fmt.Errorf("wrap: %w", New(404, "NOT_FOUND", "not found")), 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCode(tt.err); got != tt.want {
				t.Errorf("GetCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantResult *ApplicationError
	}{
		{
			"nil returns nil",
			nil,
			nil,
		},
		{
			"ApplicationError returned as-is",
			NewError(451, "FAILED", "set error pending"),
			&ApplicationError{ErrorCode: 451, Status: FailedStatus, Message: "set error pending"},
		},
		{
			"plain error wrapped in 500",
			errors.New("set error"),
			&ApplicationError{ErrorCode: http.StatusInternalServerError, Status: FailedStatus, Message: "set error"},
		},
		{
			"New() error",
			New(403, "FAILED", "error v1"),
			&ApplicationError{ErrorCode: 403, Status: FailedStatus, Message: "error v1"},
		},
		{
			"NewError without message uses StatusMessage",
			NewError(451, "FAILED"),
			&ApplicationError{ErrorCode: 451, Status: FailedStatus, Message: "Unavailable For Legal Reasons"},
		},
		{
			"NewError with unknown code uses UndefinedMessage",
			NewError(600, "FAILED"),
			&ApplicationError{ErrorCode: 600, Status: FailedStatus, Message: UndefinedMessage},
		},
		{
			"wrapped ApplicationError unwrapped via errors.As",
			fmt.Errorf("service layer: %w", New(422, "VALIDATION_ERROR", "invalid email")),
			&ApplicationError{ErrorCode: 422, Status: "VALIDATION_ERROR", Message: "invalid email"},
		},
		{
			"gRPC NotFound mapped to 404",
			status.Error(codes.NotFound, "resource not found"),
			&ApplicationError{ErrorCode: 404, Status: FailedStatus, Message: "resource not found"},
		},
		{
			"gRPC DeadlineExceeded mapped to 504",
			status.Error(codes.DeadlineExceeded, "upstream timed out"),
			&ApplicationError{ErrorCode: 504, Status: FailedStatus, Message: "upstream timed out"},
		},
		{
			"gRPC ResourceExhausted mapped to 429",
			status.Error(codes.ResourceExhausted, "rate limit hit"),
			&ApplicationError{ErrorCode: 429, Status: FailedStatus, Message: "rate limit hit"},
		},
		{
			"gRPC FailedPrecondition mapped to 400",
			status.Error(codes.FailedPrecondition, "precondition failed"),
			&ApplicationError{ErrorCode: 400, Status: FailedStatus, Message: "precondition failed"},
		},
		{
			"gRPC Aborted mapped to 409",
			status.Error(codes.Aborted, "transaction aborted"),
			&ApplicationError{ErrorCode: 409, Status: FailedStatus, Message: "transaction aborted"},
		},
		{
			"gRPC OutOfRange mapped to 400",
			status.Error(codes.OutOfRange, "index out of range"),
			&ApplicationError{ErrorCode: 400, Status: FailedStatus, Message: "index out of range"},
		},
		{
			"gRPC Unauthenticated mapped to 401",
			status.Error(codes.Unauthenticated, "missing token"),
			&ApplicationError{ErrorCode: 401, Status: FailedStatus, Message: "missing token"},
		},
		{
			"gRPC Unavailable mapped to 502",
			status.Error(codes.Unavailable, "service down"),
			&ApplicationError{ErrorCode: 502, Status: FailedStatus, Message: "service down"},
		},
		{
			"unknown gRPC code passes through as-is",
			status.Error(451, "error rpc"),
			&ApplicationError{ErrorCode: 451, Status: FailedStatus, Message: "error rpc"},
		},
		{
			"error string with '=' — last segment extracted",
			errors.New("context = deadline exceeded"),
			&ApplicationError{ErrorCode: http.StatusInternalServerError, Status: FailedStatus, Message: "deadline exceeded"},
		},
		{
			"error string without '=' — message unchanged",
			errors.New("something went wrong"),
			&ApplicationError{ErrorCode: http.StatusInternalServerError, Status: FailedStatus, Message: "something went wrong"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResult := ParseError(tt.err); !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("ParseError() = %+v, want %+v", gotResult, tt.wantResult)
			}
		})
	}
}

func TestNewError(t *testing.T) {
	tests := []struct {
		name string
		code int
		stat string
		msg  []string
		want *ApplicationError
	}{
		{
			"with explicit message",
			400, FailedStatus, []string{"Bad Request"},
			&ApplicationError{ErrorCode: 400, Status: FailedStatus, Message: "Bad Request"},
		},
		{
			"without message — uses StatusMessage",
			400, FailedStatus, nil,
			&ApplicationError{ErrorCode: 400, Status: FailedStatus, Message: "Bad Request"},
		},
		{
			"unknown code — uses UndefinedMessage",
			999, FailedStatus, nil,
			&ApplicationError{ErrorCode: 999, Status: FailedStatus, Message: UndefinedMessage},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewError(tt.code, tt.stat, tt.msg...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewError() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	err := New(400, "FAILED", "Gagal")
	if err == nil {
		t.Fatal("New() should return non-nil error")
	}
	ae, ok := err.(*ApplicationError)
	if !ok {
		t.Fatal("New() should return *ApplicationError")
	}
	if ae.ErrorCode != 400 || ae.Status != "FAILED" || ae.Message != "Gagal" {
		t.Errorf("New() fields = {%d, %q, %q}", ae.ErrorCode, ae.Status, ae.Message)
	}
}

func TestStatusMessage(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{200, "OK"},
		{404, "Not Found"},
		{500, "Internal Server error"},
		{504, "Gateway Timeout"},
		{429, "Too Many Requests"},
		{600, UndefinedMessage},
		{-1, UndefinedMessage},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
			if got := StatusMessage(tt.code); got != tt.want {
				t.Errorf("StatusMessage(%d) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"ErrDeadlineExceeded sentinel", ErrDeadlineExceeded, true},
		{"DeadlineExceededError custom message", DeadlineExceededError("custom timeout msg"), true},
		{"gRPC DeadlineExceeded", status.Error(codes.DeadlineExceeded, "deadline"), true},
		{"plain error is not timeout", errors.New("not timeout"), false},
		// context.DeadlineExceeded implements Timeout() bool → true, so os.IsTimeout catches it.
		{"context.DeadlineExceeded is detected via os.IsTimeout", context.DeadlineExceeded, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTimeout(tt.err); got != tt.want {
				t.Errorf("IsTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeadlineExceededError(t *testing.T) {
	t.Run("default message does not mention appContext", func(t *testing.T) {
		err := DeadlineExceededError()
		if err == nil {
			t.Fatal("should not be nil")
		}
		if !os.IsTimeout(err) {
			t.Error("should satisfy os.IsTimeout")
		}
		msg := err.Error()
		if msg == "" {
			t.Error("Error() should not be empty")
		}
		for i := 0; i <= len(msg)-len("appContext"); i++ {
			if msg[i:i+len("appContext")] == "appContext" {
				t.Errorf("default message should not contain 'appContext': %q", msg)
				break
			}
		}
	})

	t.Run("custom message", func(t *testing.T) {
		err := DeadlineExceededError("my custom timeout")
		if err.Error() != "my custom timeout" {
			t.Errorf("Error() = %q, want %q", err.Error(), "my custom timeout")
		}
		if !os.IsTimeout(err) {
			t.Error("custom message error should still satisfy os.IsTimeout")
		}
	})
}

func TestCodeApplication_UnknownCode(t *testing.T) {
	// gRPC codes beyond the defined range should pass through as-is.
	unknown := codes.Code(999)
	got := codeApplication(unknown)
	if got != 999 {
		t.Errorf("codeApplication(999) = %d, want 999", got)
	}
}
