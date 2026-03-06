package connecterrors_test

import (
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	connecterrors "github.com/balcieren/connect-errors-go"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		code        connecterrors.ErrorCode
		data        connecterrors.M
		wantCode    connect.Code
		wantContain string
	}{
		{"not found", connecterrors.ErrNotFound, connecterrors.M{"id": "123"}, connect.CodeNotFound, "Resource '123' not found"},
		{"invalid argument", connecterrors.ErrInvalidArgument, connecterrors.M{"reason": "email required"}, connect.CodeInvalidArgument, "Invalid argument: email required"},
		{"already exists", connecterrors.ErrAlreadyExists, connecterrors.M{"id": "a@b.com"}, connect.CodeAlreadyExists, "Resource 'a@b.com' already exists"},
		{"unauthenticated", connecterrors.ErrUnauthenticated, nil, connect.CodeUnauthenticated, "Authentication required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := connecterrors.New(tt.code, tt.data)
			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if err.Code() != tt.wantCode {
				t.Errorf("Code() = %v, want %v", err.Code(), tt.wantCode)
			}
			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("Error() = %q, should contain %q", err.Error(), tt.wantContain)
			}
			if got := err.Meta().Get("x-error-code"); got != string(tt.code) {
				t.Errorf("x-error-code = %q, want %q", got, tt.code)
			}
		})
	}
}

func TestNewUnknownCode(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrorCode("NONEXISTENT"), nil)
	if err.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got %v", err.Code())
	}
}

func TestNewRetryableMetadata(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrUnavailable, nil)
	if got := err.Meta().Get("x-retryable"); got != "true" {
		t.Errorf("Unavailable x-retryable = %q, want true", got)
	}
	err = connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "1"})
	if got := err.Meta().Get("x-retryable"); got != "false" {
		t.Errorf("NotFound x-retryable = %q, want false", got)
	}
}

func TestNewWithMessage(t *testing.T) {
	err := connecterrors.NewWithMessage(connecterrors.ErrNotFound, "User '{{id}}' gone from '{{tenant}}'", connecterrors.M{"id": "123", "tenant": "acme"})
	if !strings.Contains(err.Error(), "User '123' gone from 'acme'") {
		t.Errorf("Error() = %q, should contain message", err.Error())
	}
	if err.Code() != connect.CodeNotFound {
		t.Errorf("Code() = %v", err.Code())
	}
}

func TestNewWithMessageUnknownCode(t *testing.T) {
	err := connecterrors.NewWithMessage(connecterrors.ErrorCode("NONEXISTENT"), "msg", nil)
	if err.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got %v", err.Code())
	}
}

func TestFromCode(t *testing.T) {
	err := connecterrors.FromCode(connect.CodeInternal, "db error")
	if err.Code() != connect.CodeInternal {
		t.Errorf("Code() = %v", err.Code())
	}
	if !strings.Contains(err.Error(), "db error") {
		t.Errorf("Error() = %q, should contain 'db error'", err.Error())
	}
}

func TestWrap(t *testing.T) {
	orig := errors.New("connection refused")
	err := connecterrors.Wrap(connecterrors.ErrNotFound, orig, connecterrors.M{"id": "456"})
	if err.Code() != connect.CodeNotFound {
		t.Errorf("Code() = %v", err.Code())
	}
	msg := err.Error()
	if !strings.Contains(msg, "Resource '456' not found") {
		t.Errorf("missing template msg in %q", msg)
	}
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("missing wrapped error in %q", msg)
	}
}

func TestWrapUnknownCode(t *testing.T) {
	err := connecterrors.Wrap(connecterrors.ErrorCode("NONEXISTENT"), errors.New("fail"), nil)
	if err.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got %v", err.Code())
	}
}

func TestIsRetryable(t *testing.T) {
	if !connecterrors.IsRetryable(connecterrors.ErrUnavailable) {
		t.Error("Unavailable should be retryable")
	}
	if connecterrors.IsRetryable(connecterrors.ErrNotFound) {
		t.Error("NotFound should not be retryable")
	}
	if connecterrors.IsRetryable(connecterrors.ErrorCode("NONEXISTENT")) {
		t.Error("unknown should not be retryable")
	}
}

func TestConnectCode(t *testing.T) {
	if got := connecterrors.ConnectCode(connecterrors.ErrNotFound); got != connect.CodeNotFound {
		t.Errorf("got %v, want CodeNotFound", got)
	}
	if got := connecterrors.ConnectCode(connecterrors.ErrorCode("NONEXISTENT")); got != connect.CodeInternal {
		t.Errorf("got %v, want CodeInternal", got)
	}
}

func TestNewf(t *testing.T) {
	err := connecterrors.Newf(connecterrors.ErrNotFound, "User %q not found in org %s", "alice", "acme")
	if err.Code() != connect.CodeNotFound {
		t.Errorf("Code() = %v, want CodeNotFound", err.Code())
	}
	if !strings.Contains(err.Error(), `User "alice" not found in org acme`) {
		t.Errorf("Error() = %q, should contain formatted message", err.Error())
	}
	if got := err.Meta().Get("x-error-code"); got != string(connecterrors.ErrNotFound) {
		t.Errorf("x-error-code = %q, want %q", got, connecterrors.ErrNotFound)
	}
}

func TestNewfUnknownCode(t *testing.T) {
	err := connecterrors.Newf(connecterrors.ErrorCode("NONEXISTENT"), "msg %s", "val")
	if err.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got %v", err.Code())
	}
}

func TestFromError(t *testing.T) {
	connectErr := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "123"})
	e, ok := connecterrors.FromError(connectErr)
	if !ok {
		t.Fatal("expected FromError to find error")
	}
	if e.Code != connecterrors.ErrNotFound {
		t.Errorf("Code = %q, want %q", e.Code, connecterrors.ErrNotFound)
	}
	if e.ConnectCode != connect.CodeNotFound {
		t.Errorf("ConnectCode = %v, want CodeNotFound", e.ConnectCode)
	}
}

func TestFromErrorNil(t *testing.T) {
	_, ok := connecterrors.FromError(nil)
	if ok {
		t.Error("expected false for nil error")
	}
}

func TestFromErrorNoMeta(t *testing.T) {
	connectErr := connect.NewError(connect.CodeInternal, errors.New("raw error"))
	_, ok := connecterrors.FromError(connectErr)
	if ok {
		t.Error("expected false for error without x-error-code meta")
	}
}

func TestCodedErrorIs(t *testing.T) {
	sentinel := connecterrors.NewCodedError(connecterrors.ErrNotFound)
	connectErr := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "1"})

	if !errors.Is(connectErr.Unwrap(), sentinel) {
		t.Error("expected errors.Is to match by code")
	}

	other := connecterrors.NewCodedError(connecterrors.ErrInternal)
	if errors.Is(connectErr.Unwrap(), other) {
		t.Error("should not match different code")
	}
}

func TestCodedErrorAs(t *testing.T) {
	connectErr := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "42"})

	var coded *connecterrors.CodedError
	if !errors.As(connectErr.Unwrap(), &coded) {
		t.Fatal("expected errors.As to extract CodedError")
	}
	if coded.Code() != string(connecterrors.ErrNotFound) {
		t.Errorf("Code() = %q, want %q", coded.Code(), connecterrors.ErrNotFound)
	}
	if !strings.Contains(coded.Error(), "42") {
		t.Errorf("Error() = %q, should contain '42'", coded.Error())
	}
}

func TestWrapCodedError(t *testing.T) {
	orig := errors.New("db connection lost")
	connectErr := connecterrors.Wrap(connecterrors.ErrInternal, orig, connecterrors.M{})

	sentinel := connecterrors.NewCodedError(connecterrors.ErrInternal)
	if !errors.Is(connectErr.Unwrap(), sentinel) {
		t.Error("expected Wrap result to match sentinel via errors.Is")
	}

	// The original error should also be reachable via errors.Is
	if !errors.Is(connectErr.Unwrap(), orig) {
		t.Error("expected original error to be reachable via errors.Is")
	}
}

func TestNewfCodedError(t *testing.T) {
	connectErr := connecterrors.Newf(connecterrors.ErrNotFound, "user %s gone", "alice")
	sentinel := connecterrors.NewCodedError(connecterrors.ErrNotFound)

	if !errors.Is(connectErr.Unwrap(), sentinel) {
		t.Error("expected Newf result to match sentinel via errors.Is")
	}

	var coded *connecterrors.CodedError
	if !errors.As(connectErr.Unwrap(), &coded) {
		t.Fatal("expected errors.As to extract CodedError from Newf result")
	}
	if coded.Code() != string(connecterrors.ErrNotFound) {
		t.Errorf("Code() = %q, want %q", coded.Code(), connecterrors.ErrNotFound)
	}
}

func TestWithDetails(t *testing.T) {
	connectErr := connecterrors.New(connecterrors.ErrInvalidArgument, connecterrors.M{"reason": "bad"})
	baseLen := len(connectErr.Details())
	if baseLen != 1 {
		t.Fatalf("expected 1 detail initially (ErrorInfo), got %d", baseLen)
	}

	// WithDetails with nil should not panic
	result := connecterrors.WithDetails(connectErr, nil)
	if result != connectErr {
		t.Error("expected same error returned for chaining")
	}
	if len(connectErr.Details()) != baseLen {
		t.Error("nil detail should not be added")
	}
}

func TestExtractErrorCode(t *testing.T) {
	connectErr := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "1"})
	code, ok := connecterrors.ExtractErrorCode(connectErr)
	if !ok {
		t.Fatal("expected ErrorCode to return true")
	}
	if code != string(connecterrors.ErrNotFound) {
		t.Errorf("code = %q, want %q", code, connecterrors.ErrNotFound)
	}
}

func TestExtractErrorCodeNil(t *testing.T) {
	_, ok := connecterrors.ExtractErrorCode(nil)
	if ok {
		t.Error("expected false for nil")
	}
}

func TestExtractErrorCodeNoMeta(t *testing.T) {
	connectErr := connect.NewError(connect.CodeInternal, errors.New("raw"))
	_, ok := connecterrors.ExtractErrorCode(connectErr)
	if ok {
		t.Error("expected false for error without x-error-code")
	}
}

func TestNewWithCodedError(t *testing.T) {
	// Create a sentinel error (simulating generated code)
	sentinel := connecterrors.NewCodedError(connecterrors.ErrNotFound)

	// Use sentinel with New() - this is the new feature
	err := connecterrors.New(sentinel, connecterrors.M{"id": "123"})
	if err.Code() != connect.CodeNotFound {
		t.Errorf("Code() = %v, want CodeNotFound", err.Code())
	}
	if got := err.Meta().Get("x-error-code"); got != string(connecterrors.ErrNotFound) {
		t.Errorf("x-error-code = %q, want %q", got, connecterrors.ErrNotFound)
	}
}

func TestNewWithNilCode(t *testing.T) {
	// Passing nil should not panic, should return Internal error
	err := connecterrors.New(nil, nil)
	if err.Code() != connect.CodeInternal {
		t.Errorf("Code() = %v, want CodeInternal for nil code", err.Code())
	}
}

func TestWrapWithCodedError(t *testing.T) {
	sentinel := connecterrors.NewCodedError(connecterrors.ErrInternal)
	orig := errors.New("db error")
	err := connecterrors.Wrap(sentinel, orig, connecterrors.M{})
	if err.Code() != connect.CodeInternal {
		t.Errorf("Code() = %v, want CodeInternal", err.Code())
	}
}

func TestSetHeaderKeys(t *testing.T) {
	// Restore default keys after test
	defer func() {
		connecterrors.SetHeaderKeys("x-error-code", "x-retryable")
	}()

	connecterrors.SetHeaderKeys("x-custom-code", "x-custom-retry")

	err := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "123"})
	meta := err.Meta()

	if meta.Get("x-custom-code") != string(connecterrors.ErrNotFound) {
		t.Errorf("expected x-custom-code header to be %s, got %s", connecterrors.ErrNotFound, meta.Get("x-custom-code"))
	}

	if meta.Get("x-error-code") != "" {
		t.Errorf("expected x-error-code header to be empty, got %s", meta.Get("x-error-code"))
	}
}

// isErrorCode simulates the generated IsXxx pattern for testing.
func isErrorCode(err error, code connecterrors.ErrorCode) bool {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return false
	}
	c, ok := connecterrors.ExtractErrorCode(connectErr)
	return ok && c == string(code)
}

func TestIsXxxPatternMatch(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "42"})
	if !isErrorCode(err, connecterrors.ErrNotFound) {
		t.Error("expected IsNotFound to match")
	}
}

func TestIsXxxPatternNoMatch(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "42"})
	if isErrorCode(err, connecterrors.ErrInternal) {
		t.Error("expected IsInternal to NOT match a NotFound error")
	}
}

func TestIsXxxPatternNilError(t *testing.T) {
	if isErrorCode(nil, connecterrors.ErrNotFound) {
		t.Error("expected false for nil error")
	}
}

func TestIsXxxPatternNonConnectError(t *testing.T) {
	plainErr := errors.New("some random error")
	if isErrorCode(plainErr, connecterrors.ErrNotFound) {
		t.Error("expected false for non-connect error")
	}
}

func TestIsXxxPatternRawConnectError(t *testing.T) {
	// A raw *connect.Error without x-error-code metadata
	rawErr := connect.NewError(connect.CodeNotFound, errors.New("not found"))
	if isErrorCode(rawErr, connecterrors.ErrNotFound) {
		t.Error("expected false for connect error without x-error-code metadata")
	}
}
func TestCodedErrorErrorCodeDeprecated(t *testing.T) {
	sentinel := connecterrors.NewCodedError(connecterrors.ErrNotFound)
	// ErrorCode() is deprecated but should still work
	if sentinel.ErrorCode() != string(connecterrors.ErrNotFound) {
		t.Errorf("ErrorCode() = %q, want %q", sentinel.ErrorCode(), connecterrors.ErrNotFound)
	}
}

func TestWithMultipleDetails(t *testing.T) {
	connectErr := connecterrors.New(connecterrors.ErrInvalidArgument, connecterrors.M{"reason": "bad"})
	baseLen := len(connectErr.Details())

	// Use real proto messages for details
	d1, err := connect.NewErrorDetail(&emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	d2, err := connect.NewErrorDetail(&emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}

	connectErr = connecterrors.WithDetails(connectErr, d1, d2)

	expectedLen := baseLen + 2
	if len(connectErr.Details()) != expectedLen {
		t.Errorf("len(Details) = %d, want %d", len(connectErr.Details()), expectedLen)
	}
}

func TestExtractErrorInfo(t *testing.T) {
	connectErr := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "1"})
	info, ok := connecterrors.ExtractErrorInfo(connectErr)
	if !ok {
		t.Fatal("expected ExtractErrorInfo to return true")
	}
	if info.Reason != string(connecterrors.ErrNotFound) {
		t.Errorf("Reason = %q, want %q", info.Reason, connecterrors.ErrNotFound)
	}
	if info.Domain != "connecterrors" {
		t.Errorf("Domain = %q, want connecterrors", info.Domain)
	}
	if info.Metadata["id"] != "1" {
		t.Errorf("Metadata['id'] = %q, want '1'", info.Metadata["id"])
	}

	// Test non-connect error
	_, ok = connecterrors.ExtractErrorInfo(errors.New("plain err"))
	if ok {
		t.Error("expected false for plain error")
	}
}

func TestExtractRetryInfo(t *testing.T) {
	// ErrUnavailable is retryable
	connectErr := connecterrors.New(connecterrors.ErrUnavailable, nil)
	info, ok := connecterrors.ExtractRetryInfo(connectErr)
	if !ok {
		t.Fatal("expected ExtractRetryInfo to return true for ErrUnavailable")
	}
	if info.RetryDelay == nil || info.RetryDelay.Seconds != 0 {
		t.Errorf("unexpected RetryDelay: %v", info.RetryDelay)
	}

	// ErrNotFound is not retryable
	connectErr2 := connecterrors.New(connecterrors.ErrNotFound, nil)
	_, ok = connecterrors.ExtractRetryInfo(connectErr2)
	if ok {
		t.Error("expected false for ExtractRetryInfo on ErrNotFound")
	}
}

func TestErrorCodeCode(t *testing.T) {
	code := connecterrors.ErrorCode("TEST")
	if code.Code() != "TEST" {
		t.Errorf("ErrorCode.Code() = %q, want TEST", code.Code())
	}
}

func TestCodedErrorCodeNil(t *testing.T) {
	var e *connecterrors.CodedError
	if e.Code() != "" {
		t.Errorf("Code() = %q, want empty string for nil receiver", e.Code())
	}
}
