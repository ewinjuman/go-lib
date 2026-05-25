package apperror

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	pkgerrors "github.com/pkg/errors"
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
		{"plain error returns 500", pkgerrors.New("set error"), 500},
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
		name     string
		err      error
		wantCode int
		wantMsg  string
		wantStat string
	}{
		{
			"nil returns nil",
			nil, 0, "", "",
		},
		{
			"ApplicationError returned as-is",
			NewError(451, "FAILED", "set error pending"),
			451, "set error pending", FailedStatus,
		},
		{
			"plain error wrapped in 500",
			pkgerrors.New("set error"),
			http.StatusInternalServerError, "set error", FailedStatus,
		},
		{
			"New() error",
			New(403, "FAILED", "error v1"),
			403, "error v1", FailedStatus,
		},
		{
			"NewError without message uses StatusMessage",
			NewError(451, "FAILED"),
			451, "Unavailable For Legal Reasons", FailedStatus,
		},
		{
			"NewError with unknown code uses UndefinedMessage",
			NewError(600, "FAILED"),
			600, UndefinedMessage, FailedStatus,
		},
		{
			"wrapped ApplicationError unwrapped via errors.As",
			fmt.Errorf("service layer: %w", New(422, "VALIDATION_ERROR", "invalid email")),
			422, "invalid email", "VALIDATION_ERROR",
		},
		{
			"gRPC NotFound mapped to 404",
			status.Error(codes.NotFound, "resource not found"),
			404, "resource not found", FailedStatus,
		},
		{
			"gRPC DeadlineExceeded mapped to 504",
			status.Error(codes.DeadlineExceeded, "upstream timed out"),
			504, "upstream timed out", FailedStatus,
		},
		{
			"gRPC ResourceExhausted mapped to 429",
			status.Error(codes.ResourceExhausted, "rate limit hit"),
			429, "rate limit hit", FailedStatus,
		},
		{
			"gRPC FailedPrecondition mapped to 400",
			status.Error(codes.FailedPrecondition, "precondition failed"),
			400, "precondition failed", FailedStatus,
		},
		{
			"gRPC Aborted mapped to 409",
			status.Error(codes.Aborted, "transaction aborted"),
			409, "transaction aborted", FailedStatus,
		},
		{
			"gRPC OutOfRange mapped to 400",
			status.Error(codes.OutOfRange, "index out of range"),
			400, "index out of range", FailedStatus,
		},
		{
			"gRPC Unauthenticated mapped to 401",
			status.Error(codes.Unauthenticated, "missing token"),
			401, "missing token", FailedStatus,
		},
		{
			"gRPC Unavailable mapped to 502",
			status.Error(codes.Unavailable, "service down"),
			502, "service down", FailedStatus,
		},
		{
			"unknown gRPC code passes through as-is",
			status.Error(451, "error rpc"),
			451, "error rpc", FailedStatus,
		},
		{
			"error string with '=' — last segment extracted",
			pkgerrors.New("context = deadline exceeded"),
			http.StatusInternalServerError, "deadline exceeded", FailedStatus,
		},
		{
			"error string without '=' — message unchanged",
			pkgerrors.New("something went wrong"),
			http.StatusInternalServerError, "something went wrong", FailedStatus,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseError(tt.err)
			if tt.err == nil {
				if got != nil {
					t.Errorf("ParseError(nil) = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseError() returned nil, want ErrorCode=%d", tt.wantCode)
			}
			if got.ErrorCode != tt.wantCode {
				t.Errorf("ErrorCode = %d, want %d", got.ErrorCode, tt.wantCode)
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
			}
			if got.Status != tt.wantStat {
				t.Errorf("Status = %q, want %q", got.Status, tt.wantStat)
			}
		})
	}
}

func TestNewError(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		stat     string
		msg      []string
		wantCode int
		wantMsg  string
	}{
		{
			"with explicit message",
			400, FailedStatus, []string{"Bad Request"},
			400, "Bad Request",
		},
		{
			"without message — uses StatusMessage",
			400, FailedStatus, nil,
			400, "Bad Request",
		},
		{
			"unknown code — uses UndefinedMessage",
			999, FailedStatus, nil,
			999, UndefinedMessage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NewError(tt.code, tt.stat, tt.msg...).(*ApplicationError)
			if !ok {
				t.Fatal("NewError() should return *ApplicationError")
			}
			if got.ErrorCode != tt.wantCode {
				t.Errorf("ErrorCode = %d, want %d", got.ErrorCode, tt.wantCode)
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
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
		{"plain error is not timeout", pkgerrors.New("not timeout"), false},
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

// ── WithCause / Cause / Unwrap ────────────────────────────────────────────────

func TestWithCause_RetainsFields(t *testing.T) {
	base := NotFound("user not found")
	cause := errors.New("sql: no rows")
	wrapped := base.WithCause(cause)

	if wrapped.ErrorCode != 404 {
		t.Errorf("ErrorCode = %d, want 404", wrapped.ErrorCode)
	}
	if wrapped.Message != "user not found" {
		t.Errorf("Message = %q, want %q", wrapped.Message, "user not found")
	}
	if wrapped.Cause() != cause {
		t.Errorf("Cause() = %v, want %v", wrapped.Cause(), cause)
	}
}

func TestWithCause_DoesNotMutateOriginal(t *testing.T) {
	base := NotFound()
	_ = base.WithCause(errors.New("some cause"))

	if base.cause != nil {
		t.Error("WithCause should not mutate the original error")
	}
}

func TestUnwrap_TraversesChain(t *testing.T) {
	root := errors.New("root cause")
	ae := InternalError("something failed").WithCause(root)

	if !errors.Is(ae, root) {
		t.Error("errors.Is should find root cause through Unwrap chain")
	}
}

// ── Is (errors.Is matching) ───────────────────────────────────────────────────

func TestIs_MatchesBySentinel(t *testing.T) {
	err := NotFound("custom message")

	if !errors.Is(err, ErrNotFound) {
		t.Error("errors.Is(NotFound(...), ErrNotFound) should return true")
	}
	if errors.Is(err, ErrBadRequest) {
		t.Error("errors.Is(NotFound(...), ErrBadRequest) should return false")
	}
}

func TestIs_WrappedInFmtErrorf(t *testing.T) {
	inner := NotFound("item not found")
	wrapped := fmt.Errorf("service: %w", inner)

	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("errors.Is should detect ErrNotFound through fmt.Errorf wrapping")
	}
}

func TestIs_DifferentMessage_StillMatches(t *testing.T) {
	// Two errors with same code+status but different messages should match.
	a := NotFound("user not found")
	b := NotFound("product not found")

	if !errors.Is(a, b) {
		t.Error("ApplicationErrors with same code+status should match regardless of message")
	}
}

// ── Named predicates ──────────────────────────────────────────────────────────

func TestPredicates(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		predicate func(error) bool
		want      bool
	}{
		{"IsNotFound true", NotFound(), IsNotFound, true},
		{"IsNotFound false", BadRequest(), IsNotFound, false},
		{"IsBadRequest true", BadRequest(), IsBadRequest, true},
		{"IsUnauthorized true", Unauthorized(), IsUnauthorized, true},
		{"IsForbidden true", Forbidden(), IsForbidden, true},
		{"IsConflict true", Conflict(), IsConflict, true},
		{"IsUnprocessable true", UnprocessableEntity(), IsUnprocessable, true},
		{"IsTooManyRequests true", TooManyRequests(), IsTooManyRequests, true},
		{"IsInternalError true", InternalError(), IsInternalError, true},
		{"IsServiceUnavailable true", ServiceUnavailable(), IsServiceUnavailable, true},
		// wrapped error
		{"IsNotFound wrapped", fmt.Errorf("wrap: %w", NotFound("x")), IsNotFound, true},
		// plain error returns false
		{"IsNotFound plain error", errors.New("oops"), IsNotFound, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.predicate(tt.err); got != tt.want {
				t.Errorf("predicate(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ── Named constructors ────────────────────────────────────────────────────────

func TestNamedConstructors_DefaultMessage(t *testing.T) {
	tests := []struct {
		name    string
		err     *ApplicationError
		wantCode int
		wantMsg string
	}{
		{"BadRequest", BadRequest(), 400, "Bad Request"},
		{"Unauthorized", Unauthorized(), 401, "Unauthorized"},
		{"Forbidden", Forbidden(), 403, "Forbidden"},
		{"NotFound", NotFound(), 404, "Not Found"},
		{"Conflict", Conflict(), 409, "Conflict"},
		{"UnprocessableEntity", UnprocessableEntity(), 422, "Unprocessable Entity"},
		{"TooManyRequests", TooManyRequests(), 429, "Too Many Requests"},
		{"InternalError", InternalError(), 500, "Internal Server error"},
		{"ServiceUnavailable", ServiceUnavailable(), 503, "Service Unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.ErrorCode != tt.wantCode {
				t.Errorf("ErrorCode = %d, want %d", tt.err.ErrorCode, tt.wantCode)
			}
			if tt.err.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", tt.err.Message, tt.wantMsg)
			}
		})
	}
}

func TestNamedConstructors_CustomMessage(t *testing.T) {
	err := NotFound("product id=42 not found")
	if err.Message != "product id=42 not found" {
		t.Errorf("Message = %q, want custom message", err.Message)
	}
	if err.ErrorCode != 404 {
		t.Errorf("ErrorCode = %d, want 404", err.ErrorCode)
	}
}

// ── ToGRPCStatus ──────────────────────────────────────────────────────────────

func TestToGRPCStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      *ApplicationError
		wantCode codes.Code
		wantMsg  string
	}{
		{"404 → NotFound", NotFound("item missing"), codes.NotFound, "item missing"},
		{"400 → InvalidArgument", BadRequest("bad input"), codes.InvalidArgument, "bad input"},
		{"401 → Unauthenticated", Unauthorized("no token"), codes.Unauthenticated, "no token"},
		{"403 → PermissionDenied", Forbidden("denied"), codes.PermissionDenied, "denied"},
		{"409 → Aborted", Conflict("conflict"), codes.Aborted, "conflict"},
		{"422 → InvalidArgument", UnprocessableEntity("invalid"), codes.InvalidArgument, "invalid"},
		{"429 → ResourceExhausted", TooManyRequests("slow down"), codes.ResourceExhausted, "slow down"},
		{"500 → Internal", InternalError("oops"), codes.Internal, "oops"},
		{"503 → Unavailable", ServiceUnavailable("down"), codes.Unavailable, "down"},
		{"504 → DeadlineExceeded", build(504, FailedStatus, "timed out"), codes.DeadlineExceeded, "timed out"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := tt.err.ToGRPCStatus()
			if st.Code() != tt.wantCode {
				t.Errorf("Code = %v, want %v", st.Code(), tt.wantCode)
			}
			if st.Message() != tt.wantMsg {
				t.Errorf("Message = %q, want %q", st.Message(), tt.wantMsg)
			}
		})
	}
}

func TestToGRPCStatus_UnmappedCode(t *testing.T) {
	// An HTTP code with no gRPC mapping should fall back to codes.Unknown.
	err := build(418, FailedStatus, "I'm a teapot")
	st := err.ToGRPCStatus()
	if st.Code() != codes.Unknown {
		t.Errorf("Code = %v, want Unknown", st.Code())
	}
}

// ── httpCodeToGRPCCode ────────────────────────────────────────────────────────

func TestHttpCodeToGRPCCode_UnknownFallback(t *testing.T) {
	got := httpCodeToGRPCCode(999)
	if got != codes.Unknown {
		t.Errorf("httpCodeToGRPCCode(999) = %v, want Unknown", got)
	}
}
