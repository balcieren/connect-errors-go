package connecterrors_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	connecterrors "github.com/balcieren/connect-errors-go"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
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
		{"not found", connecterrors.ErrNotFound, nil, connect.CodeNotFound, "Resource not found"},
		{"invalid argument", connecterrors.ErrInvalidArgument, nil, connect.CodeInvalidArgument, "Invalid argument"},
		{"already exists", connecterrors.ErrAlreadyExists, nil, connect.CodeAlreadyExists, "Resource already exists"},
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
	err = connecterrors.New(connecterrors.ErrNotFound, nil)
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
	err := connecterrors.Wrap(connecterrors.ErrNotFound, orig, nil)
	if err.Code() != connect.CodeNotFound {
		t.Errorf("Code() = %v", err.Code())
	}
	msg := err.Error()
	if !strings.Contains(msg, "Resource not found") {
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

func TestIsRetryableError(t *testing.T) {
	retryableErr := connecterrors.New(connecterrors.ErrUnavailable, nil)
	if !connecterrors.IsRetryable(retryableErr) {
		t.Error("Unavailable error should be retryable")
	}

	nonRetryableErr := connecterrors.New(connecterrors.ErrNotFound, nil)
	if connecterrors.IsRetryable(nonRetryableErr) {
		t.Error("NotFound error should not be retryable")
	}

	plainErr := errors.New("plain error")
	if connecterrors.IsRetryable(plainErr) {
		t.Error("plain error should not be retryable")
	}
}

func TestErrorCodeImplementsError(t *testing.T) {
	var err error = connecterrors.ErrNotFound
	if err.Error() != "ERROR_NOT_FOUND" {
		t.Errorf("Error() = %q, want 'ERROR_NOT_FOUND'", err.Error())
	}
}

func TestErrorsIsErrorCode(t *testing.T) {
	connectErr := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "42"})
	if !errors.Is(connectErr, connecterrors.ErrNotFound) {
		t.Error("expected errors.Is(err, ErrNotFound) to return true")
	}
}

func TestErrorsIsErrorCodeNoMatch(t *testing.T) {
	connectErr := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "42"})
	if errors.Is(connectErr, connecterrors.ErrInternal) {
		t.Error("expected errors.Is(err, ErrInternal) to return false for NotFound error")
	}
}

func TestErrorsIsErrorCodePlainError(t *testing.T) {
	if errors.Is(errors.New("plain"), connecterrors.ErrNotFound) {
		t.Error("expected false for plain error")
	}
}

func TestResetRegistry(t *testing.T) {
	connecterrors.Register(connecterrors.Error{
		Code:        "ERROR_TEMP",
		MessageTpl:  "temp",
		ConnectCode: connect.CodeInternal,
	})
	if _, ok := connecterrors.Lookup("ERROR_TEMP"); !ok {
		t.Fatal("expected custom error to be registered")
	}

	connecterrors.ResetRegistry()

	if _, ok := connecterrors.Lookup("ERROR_TEMP"); ok {
		t.Error("expected custom error to be cleared after ResetRegistry")
	}
	if _, ok := connecterrors.Lookup(connecterrors.ErrNotFound); !ok {
		t.Error("expected default errors to still be present after ResetRegistry")
	}
}

func TestSetDomain(t *testing.T) {
	defer connecterrors.SetDomain("connecterrors")

	connecterrors.SetDomain("myapp")
	err := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "1"})
	info, ok := connecterrors.ExtractErrorInfo(err)
	if !ok {
		t.Fatal("expected ExtractErrorInfo to return true")
	}
	if info.Domain != "myapp" {
		t.Errorf("Domain = %q, want 'myapp'", info.Domain)
	}
}

func TestNewWithRetry(t *testing.T) {
	err := connecterrors.NewWithRetry(connecterrors.ErrUnavailable, nil, 5*time.Second)
	if err.Code() != connect.CodeUnavailable {
		t.Errorf("Code() = %v, want CodeUnavailable", err.Code())
	}
	retryInfo, ok := connecterrors.ExtractRetryInfo(err)
	if !ok {
		t.Fatal("expected ExtractRetryInfo to return true")
	}
	if retryInfo.RetryDelay.AsDuration() != 5*time.Second {
		t.Errorf("RetryDelay = %v, want 5s", retryInfo.RetryDelay.AsDuration())
	}
}

func TestWithFieldViolation(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrInvalidArgument, connecterrors.M{"reason": "bad"})
	connecterrors.WithFieldViolation(err, "email", "already registered")

	found := false
	for _, detail := range err.Details() {
		val, err := detail.Value()
		if err != nil {
			continue
		}
		if badRequest, ok := val.(*errdetails.BadRequest); ok {
			if len(badRequest.FieldViolations) > 0 {
				if badRequest.FieldViolations[0].Field == "email" {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Error("expected to find BadRequest FieldViolation for 'email'")
	}
}

func TestErrorBuilder(t *testing.T) {
	err := connecterrors.NewBuilder(connecterrors.ErrInvalidArgument, connecterrors.M{"reason": "bad"}).
		WithFieldViolation("email", "already registered").
		Build()

	if err.Code() != connect.CodeInvalidArgument {
		t.Errorf("Code() = %v, want CodeInvalidArgument", err.Code())
	}
}

func TestErrorBuilderWithRetryDelay(t *testing.T) {
	err := connecterrors.NewBuilder(connecterrors.ErrUnavailable, nil).
		WithRetryDelay(10 * time.Second).
		Build()

	if err.Code() != connect.CodeUnavailable {
		t.Errorf("Code() = %v, want CodeUnavailable", err.Code())
	}
	retryInfo, ok := connecterrors.ExtractRetryInfo(err)
	if !ok {
		t.Fatal("expected ExtractRetryInfo to return true")
	}
	if retryInfo.RetryDelay.AsDuration() != 10*time.Second {
		t.Errorf("RetryDelay = %v, want 10s", retryInfo.RetryDelay.AsDuration())
	}
}

func TestNewCtx(t *testing.T) {
	connecterrors.SetContextExtractor(func(ctx context.Context) connecterrors.M {
		return connecterrors.M{"trace_id": "abc123"}
	})
	defer connecterrors.SetContextExtractor(func(context.Context) connecterrors.M { return nil })

	ctx := context.WithValue(context.Background(), "test", true)
	err := connecterrors.NewCtx(ctx, connecterrors.ErrNotFound, connecterrors.M{"id": "42"})

	info, ok := connecterrors.ExtractErrorInfo(err)
	if !ok {
		t.Fatal("expected ExtractErrorInfo to return true")
	}
	if info.Metadata["trace_id"] != "abc123" {
		t.Errorf("Metadata['trace_id'] = %q, want 'abc123'", info.Metadata["trace_id"])
	}
	if info.Metadata["id"] != "42" {
		t.Errorf("Metadata['id'] = %q, want '42'", info.Metadata["id"])
	}
}

func TestToProblemDetails(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "123"})
	pd := connecterrors.ToProblemDetails(err)
	if pd == nil {
		t.Fatal("expected non-nil ProblemDetails")
	}
	if pd.Status != 404 {
		t.Errorf("Status = %d, want 404", pd.Status)
	}
	if pd.Title != string(connecterrors.ErrNotFound) {
		t.Errorf("Title = %q, want %q", pd.Title, connecterrors.ErrNotFound)
	}
}

func TestToProblemDetailsNonConnect(t *testing.T) {
	pd := connecterrors.ToProblemDetails(errors.New("plain"))
	if pd != nil {
		t.Error("expected nil for plain error")
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



func TestCodedErrorAs(t *testing.T) {
	connectErr := connecterrors.New(connecterrors.ErrNotFound, nil)

	var coded *connecterrors.CodedError
	if !errors.As(connectErr.Unwrap(), &coded) {
		t.Fatal("expected errors.As to extract CodedError")
	}
	if coded.Code() != string(connecterrors.ErrNotFound) {
		t.Errorf("Code() = %q, want %q", coded.Code(), connecterrors.ErrNotFound)
	}
	if !strings.Contains(coded.Error(), "Resource not found") {
		t.Errorf("Error() = %q, should contain 'Resource not found'", coded.Error())
	}
}

func TestWrapCodedError(t *testing.T) {
	orig := errors.New("db connection lost")
	connectErr := connecterrors.Wrap(connecterrors.ErrInternal, orig, connecterrors.M{})

	var coded *connecterrors.CodedError
	if !errors.As(connectErr.Unwrap(), &coded) {
		t.Fatal("expected errors.As to extract CodedError")
	}
	if coded.Code() != string(connecterrors.ErrInternal) {
		t.Errorf("Code() = %q, want %q", coded.Code(), connecterrors.ErrInternal)
	}

	// The original error should also be reachable via errors.Is
	if !errors.Is(connectErr.Unwrap(), orig) {
		t.Error("expected original error to be reachable via errors.Is")
	}
}

func TestNewfCodedError(t *testing.T) {
	connectErr := connecterrors.Newf(connecterrors.ErrNotFound, "user %s gone", "alice")

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



func TestNewWithNilCode(t *testing.T) {
	// Passing nil should not panic, should return Internal error
	err := connecterrors.New(nil, nil)
	if err.Code() != connect.CodeInternal {
		t.Errorf("Code() = %v, want CodeInternal for nil code", err.Code())
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
	connectErr := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "1"})
	var coded *connecterrors.CodedError
	if errors.As(connectErr.Unwrap(), &coded) {
		if coded.ErrorCode() != string(connecterrors.ErrNotFound) {
			t.Errorf("ErrorCode() = %q, want %q", coded.ErrorCode(), connecterrors.ErrNotFound)
		}
	} else {
		t.Fatal("failed to extract coded error")
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
	connectErr2 := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "1"})
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

func TestNewValidateTemplateLogger(t *testing.T) {
	connecterrors.Register(connecterrors.Error{
		Code:        "ERROR_LOGGER_TEST",
		MessageTpl:  "User '{{id}}' not found",
		ConnectCode: connect.CodeNotFound,
	})
	defer connecterrors.ResetRegistry()

	var loggedCode string
	var loggedErr error
	connecterrors.SetValidationLogger(func(code string, data connecterrors.M, err error) {
		loggedCode = code
		loggedErr = err
	})
	defer connecterrors.SetValidationLogger(nil)

	// Should not panic, should call validation logger
	err := connecterrors.New(connecterrors.ErrorCode("ERROR_LOGGER_TEST"), connecterrors.M{})
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if loggedCode != "ERROR_LOGGER_TEST" {
		t.Errorf("expected logger to be called with code ERROR_LOGGER_TEST, got %q", loggedCode)
	}
	if loggedErr == nil {
		t.Error("expected logger to be called with error")
	}
}

func TestNewWithMessageValidateTemplateLogger(t *testing.T) {
	var loggedCode string
	var loggedErr error
	connecterrors.SetValidationLogger(func(code string, data connecterrors.M, err error) {
		loggedCode = code
		loggedErr = err
	})
	defer connecterrors.SetValidationLogger(nil)

	// Should not panic, should call validation logger
	err := connecterrors.NewWithMessage(connecterrors.ErrNotFound, "User '{{id}}'", connecterrors.M{})
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if loggedCode != string(connecterrors.ErrNotFound) {
		t.Errorf("expected logger to be called with code ERROR_NOT_FOUND, got %q", loggedCode)
	}
	if loggedErr == nil {
		t.Error("expected logger to be called with error")
	}
}

func TestWrapValidateTemplateLogger(t *testing.T) {
	connecterrors.Register(connecterrors.Error{
		Code:        "ERROR_LOGGER_WRAP",
		MessageTpl:  "User '{{id}}' not found",
		ConnectCode: connect.CodeNotFound,
	})
	defer connecterrors.ResetRegistry()

	var loggedCode string
	var loggedErr error
	connecterrors.SetValidationLogger(func(code string, data connecterrors.M, err error) {
		loggedCode = code
		loggedErr = err
	})
	defer connecterrors.SetValidationLogger(nil)

	// Should not panic, should call validation logger
	err := connecterrors.Wrap(connecterrors.ErrorCode("ERROR_LOGGER_WRAP"), errors.New("db err"), connecterrors.M{})
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if loggedCode != "ERROR_LOGGER_WRAP" {
		t.Errorf("expected logger to be called with code ERROR_LOGGER_WRAP, got %q", loggedCode)
	}
	if loggedErr == nil {
		t.Error("expected logger to be called with error")
	}
}

func TestErrorLogger(t *testing.T) {
	var loggedCode string
	var loggedConnectCode connect.Code
	var loggedRetryable bool
	connecterrors.SetErrorLogger(func(code string, connectCode connect.Code, retryable bool, data connecterrors.M) {
		loggedCode = code
		loggedConnectCode = connectCode
		loggedRetryable = retryable
	})
	defer connecterrors.SetErrorLogger(nil)

	connecterrors.New(connecterrors.ErrNotFound, nil)

	if loggedCode != string(connecterrors.ErrNotFound) {
		t.Errorf("expected logger to be called with code ERROR_NOT_FOUND, got %q", loggedCode)
	}
	if loggedConnectCode != connect.CodeNotFound {
		t.Errorf("expected connect code CodeNotFound, got %v", loggedConnectCode)
	}
	if loggedRetryable != false {
		t.Error("expected retryable to be false")
	}
}

func TestErrorLoggerRetryable(t *testing.T) {
	var loggedRetryable bool
	connecterrors.SetErrorLogger(func(code string, connectCode connect.Code, retryable bool, data connecterrors.M) {
		loggedRetryable = retryable
	})
	defer connecterrors.SetErrorLogger(nil)

	connecterrors.New(connecterrors.ErrUnavailable, nil)

	if loggedRetryable != true {
		t.Error("expected retryable to be true for ErrUnavailable")
	}
}

func TestNewUnknownCodeSetsMetadata(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrorCode("NONEXISTENT"), nil)
	if err.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got %v", err.Code())
	}
	code, ok := connecterrors.ExtractErrorCode(err)
	if !ok {
		t.Fatal("expected ExtractErrorCode to return true for unknown code with metadata")
	}
	if code != "NONEXISTENT" {
		t.Errorf("code = %q, want NONEXISTENT", code)
	}
}

func TestNewWithMessageUnknownCodeSetsMetadata(t *testing.T) {
	err := connecterrors.NewWithMessage(connecterrors.ErrorCode("NONEXISTENT"), "msg", nil)
	if err.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got %v", err.Code())
	}
	code, ok := connecterrors.ExtractErrorCode(err)
	if !ok {
		t.Fatal("expected ExtractErrorCode to return true for unknown code with metadata")
	}
	if code != "NONEXISTENT" {
		t.Errorf("code = %q, want NONEXISTENT", code)
	}
}

func TestWrapUnknownCodeSetsMetadata(t *testing.T) {
	err := connecterrors.Wrap(connecterrors.ErrorCode("NONEXISTENT"), errors.New("fail"), nil)
	if err.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got %v", err.Code())
	}
	code, ok := connecterrors.ExtractErrorCode(err)
	if !ok {
		t.Fatal("expected ExtractErrorCode to return true for unknown code with metadata")
	}
	if code != "NONEXISTENT" {
		t.Errorf("code = %q, want NONEXISTENT", code)
	}
}

func TestNewfUnknownCodeSetsMetadata(t *testing.T) {
	err := connecterrors.Newf(connecterrors.ErrorCode("NONEXISTENT"), "msg %s", "val")
	if err.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got %v", err.Code())
	}
	code, ok := connecterrors.ExtractErrorCode(err)
	if !ok {
		t.Fatal("expected ExtractErrorCode to return true for unknown code with metadata")
	}
	if code != "NONEXISTENT" {
		t.Errorf("code = %q, want NONEXISTENT", code)
	}
}

func TestMatchesError(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "42"})

	if !connecterrors.MatchesError(err, connecterrors.ErrNotFound) {
		t.Error("expected MatchesError to return true for matching code")
	}
	if connecterrors.MatchesError(err, connecterrors.ErrInternal) {
		t.Error("expected MatchesError to return false for non-matching code")
	}
}

func TestMatchesErrorNonConnect(t *testing.T) {
	if connecterrors.MatchesError(errors.New("plain"), connecterrors.ErrNotFound) {
		t.Error("expected false for plain error")
	}
}

func TestMatchesErrorNil(t *testing.T) {
	if connecterrors.MatchesError(nil, connecterrors.ErrNotFound) {
		t.Error("expected false for nil error")
	}
}

func TestMatchError(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "42"})

	result, ok := connecterrors.MatchError(err, []connecterrors.Matcher[string]{
		{Code: connecterrors.ErrNotFound, Fn: func() string { return "not found" }},
		{Code: connecterrors.ErrInvalidArgument, Fn: func() string { return "bad input" }},
	})
	if !ok {
		t.Fatal("expected MatchError to find a match")
	}
	if result != "not found" {
		t.Errorf("result = %q, want 'not found'", result)
	}
}

func TestMatchErrorNoMatch(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "42"})

	_, ok := connecterrors.MatchError(err, []connecterrors.Matcher[string]{
		{Code: connecterrors.ErrInternal, Fn: func() string { return "internal" }},
	})
	if ok {
		t.Error("expected no match")
	}
}

func TestMatchErrorNonConnect(t *testing.T) {
	_, ok := connecterrors.MatchError(errors.New("plain"), []connecterrors.Matcher[string]{
		{Code: connecterrors.ErrNotFound, Fn: func() string { return "not found" }},
	})
	if ok {
		t.Error("expected no match for plain error")
	}
}

func TestMatchErrorDeterministicOrder(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "42"})

	// First matcher should win even if multiple could match
	result, ok := connecterrors.MatchError(err, []connecterrors.Matcher[string]{
		{Code: connecterrors.ErrNotFound, Fn: func() string { return "first" }},
		{Code: connecterrors.ErrNotFound, Fn: func() string { return "second" }},
	})
	if !ok {
		t.Fatal("expected match")
	}
	if result != "first" {
		t.Errorf("result = %q, want 'first' (deterministic order)", result)
	}
}

func TestErrorBuilderWithDomain(t *testing.T) {
	err := connecterrors.NewBuilder(connecterrors.ErrNotFound, connecterrors.M{"id": "123"}).
		WithDomain("myapp").
		Build()

	info, ok := connecterrors.ExtractErrorInfo(err)
	if !ok {
		t.Fatal("expected ExtractErrorInfo to return true")
	}
	if info.Domain != "myapp" {
		t.Errorf("Domain = %q, want 'myapp'", info.Domain)
	}
	// Global domain should not be affected
	if connecterrors.GetDomain() != "connecterrors" {
		t.Errorf("global domain changed to %q, want 'connecterrors'", connecterrors.GetDomain())
	}
}

func TestErrorBuilderWithMessage(t *testing.T) {
	err := connecterrors.NewBuilder(connecterrors.ErrNotFound, connecterrors.M{"id": "42"}).
		WithMessage("Custom '{{id}}' error message").
		Build()

	if !strings.Contains(err.Error(), "Custom '42' error message") {
		t.Errorf("Error() = %q, should contain custom message", err.Error())
	}
}

func TestErrorBuilderWithDetail(t *testing.T) {
	detail, derr := connect.NewErrorDetail(&emptypb.Empty{})
	if derr != nil {
		t.Fatal(derr)
	}
	builtErr := connecterrors.NewBuilder(connecterrors.ErrNotFound, nil).
		WithDetail(detail).
		Build()

	// Base detail (ErrorInfo) + custom detail = 2
	if len(builtErr.Details()) < 2 {
		t.Errorf("expected >= 2 details, got %d", len(builtErr.Details()))
	}
}

func TestIsRetryableConnectError(t *testing.T) {
	// Test the *connect.Error case (third switch branch in IsRetryable)
	err := connecterrors.New(connecterrors.ErrUnavailable, nil)
	if !connecterrors.IsRetryable(err) {
		t.Error("expected IsRetryable to return true for ErrUnavailable")
	}
}

func TestIsRetryableNonRetryableConnectError(t *testing.T) {
	err := connecterrors.New(connecterrors.ErrNotFound, nil)
	if connecterrors.IsRetryable(err) {
		t.Error("expected IsRetryable to return false for ErrNotFound")
	}
}

func TestConcurrentSetErrorLoggerAndNew(t *testing.T) {
	done := make(chan struct{})

	// Writer goroutine: repeatedly changes the logger
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			connecterrors.SetErrorLogger(func(code string, connectCode connect.Code, retryable bool, data connecterrors.M) {})
		}
	}()

	// Reader goroutine: repeatedly creates errors (reads logger)
	for i := 0; i < 1000; i++ {
		connecterrors.New(connecterrors.ErrNotFound, nil)
	}

	<-done
}

func TestConcurrentSetHeaderKeys(t *testing.T) {
	defer connecterrors.SetHeaderKeys("x-error-code", "x-retryable")

	done := make(chan struct{}, 2)

	// Two goroutines writing concurrently
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 1000; i++ {
			connecterrors.SetHeaderKeys("x-app-code", "x-app-retry")
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 1000; i++ {
			connecterrors.SetHeaderKeys("x-other-code", "x-other-retry")
		}
	}()

	// Wait for both writers
	<-done
	<-done

	// Verify final state is one of the two valid options
	hk := connecterrors.GetHeaderKeys()
	ek := hk.ErrorCode()
	if ek != "x-app-code" && ek != "x-other-code" {
		t.Errorf("unexpected final errorCode key: %q", ek)
	}
}
