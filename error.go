package connecterrors

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connectproto"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ErrorCoder is the interface accepted by all error creation functions.
// Both ErrorCode (named string) and *CodedError satisfy this interface.
type ErrorCoder interface {
	Code() string
}

// ErrorCode is a type-safe error code identifier.
// Use this instead of raw strings for compile-time safety.
// ErrorCode implements the error interface, enabling errors.Is matching:
//
//	const MyError cerr.ErrorCode = "ERROR_CUSTOM"
//	cerr.New(MyError, cerr.M{"key": "val"})
//	errors.Is(err, MyError) // true if err was created with MyError
type ErrorCode string

// Code returns the string representation of the error code.
// This method satisfies the ErrorCoder interface.
func (c ErrorCode) Code() string { return string(c) }

// Error implements the error interface, allowing ErrorCode to be used
// directly as an error sentinel for errors.Is comparisons.
func (c ErrorCode) Error() string { return string(c) }

// M is a shorthand type for template data maps.
// Keys are placeholder names, values are their replacements.
//
// Example:
//
//	connecterrors.M{"id": "123", "email": "user@example.com"}
type M map[string]string

// ErrorLogger is called for every error creation (New, NewWithMessage, Wrap, etc.).
// Default: no-op (no logging).
//
// Example:
//
//	cerr.SetErrorLogger(func(code string, statusCode connect.Code, retryable bool, data cerr.M) {
//	    slog.Info("error created", "code", code, "status_code", statusCode)
//	})
type ErrorLogger func(code string, statusCode connect.Code, retryable bool, data M)

// ValidationLogger is called when template validation fails.
// Default: no-op (no logging, no panic).
//
// Example:
//
//	cerr.SetValidationLogger(func(code string, data cerr.M, err error) {
//	    slog.Error("template validation failed", "code", code, "error", err)
//	})
type ValidationLogger func(code string, data M, err error)

// errorLoggerVal stores the current error logger atomically. Default: no-op.
var errorLoggerVal atomic.Value

// validationLoggerVal stores the current validation logger atomically. Default: no-op.
var validationLoggerVal atomic.Value

// getErrorLogger returns the current error logger.
func getErrorLogger() ErrorLogger {
	return errorLoggerVal.Load().(ErrorLogger)
}

// getValidationLogger returns the current validation logger.
func getValidationLogger() ValidationLogger {
	return validationLoggerVal.Load().(ValidationLogger)
}

// SetErrorLogger configures a custom logger for all error creations.
// This is safe for concurrent use.
//
// Example:
//
//	// Zap integration
//	logger, _ := zap.NewProduction()
//	cerr.SetErrorLogger(func(code string, statusCode connect.Code, retryable bool, data cerr.M) {
//	    logger.Info("error created",
//	        zap.String("code", code),
//	        zap.String("status_code", statusCode.String()),
//	        zap.Bool("retryable", retryable),
//	    )
//	})
//
//	// Sentry integration
//	cerr.SetErrorLogger(func(code string, statusCode connect.Code, retryable bool, data cerr.M) {
//	    sentry.WithScope(func(scope *sentry.Scope) {
//	        scope.SetTag("error_code", code)
//	        sentry.CaptureMessage("Error created: " + code)
//	    })
//	})
func SetErrorLogger(fn ErrorLogger) {
	if fn == nil {
		fn = func(string, connect.Code, bool, M) {}
	}
	errorLoggerVal.Store(fn)
}

// SetValidationLogger configures a custom logger for template validation failures.
// This is safe for concurrent use.
//
// Example:
//
//	cerr.SetValidationLogger(func(code string, data cerr.M, err error) {
//	    slog.Error("template validation failed", "code", code, "error", err)
//	})
func SetValidationLogger(fn ValidationLogger) {
	if fn == nil {
		fn = func(string, M, error) {}
	}
	validationLoggerVal.Store(fn)
}

// domainVal stores the configured error domain atomically.
var domainVal atomic.Value

func init() {
	domainVal.Store("connecterrors")
	errorLoggerVal.Store(ErrorLogger(func(code string, statusCode connect.Code, retryable bool, data M) {}))
	validationLoggerVal.Store(ValidationLogger(func(code string, data M, err error) {}))
}

// GetDomain returns the configured error domain used in ErrorInfo details.
func GetDomain() string {
	return domainVal.Load().(string)
}

// SetDomain configures the error domain used in google.rpc.ErrorInfo details.
// Default is "connecterrors". This is safe for concurrent use.
//
// Example:
//
//	cerr.SetDomain("myapp")
func SetDomain(domain string) {
	if domain != "" {
		domainVal.Store(domain)
	}
}

// withMeta returns a copy of connectErr carrying the google.rpc.ErrorInfo detail
// (reason = error code, metadata = template data) and, for retryable errors, a
// google.rpc.RetryInfo detail. Connect v2 errors carry no metadata headers, so
// these details are the single source of truth for the error code and the
// retryable flag on both the server and the client.
// If retryDelay is non-zero, it is used in RetryInfo instead of the default zero delay.
// If domainOverride is non-empty, it is used instead of the global domain.
func withMeta(connectErr *connect.Error, code string, retryable bool, data M, retryDelay time.Duration, domainOverride string) *connect.Error {
	domain := domainOverride
	if domain == "" {
		domain = GetDomain()
	}
	info := &errdetails.ErrorInfo{
		Reason: code,
		Domain: domain,
	}
	if len(data) > 0 {
		info.Metadata = make(map[string]string, len(data))
		for k, v := range data {
			info.Metadata[k] = v
		}
	}
	if detail, err := connectproto.NewErrorDetail(info); err == nil {
		connectErr = connectErr.WithDetail(detail)
	}

	if retryable {
		retryInfo := &errdetails.RetryInfo{
			RetryDelay: durationpb.New(retryDelay),
		}
		if detail, err := connectproto.NewErrorDetail(retryInfo); err == nil {
			connectErr = connectErr.WithDetail(detail)
		}
	}
	return connectErr
}

// FromError resolves the Error definition of a *connect.Error from the Registry,
// using the domain error code carried in its google.rpc.ErrorInfo detail.
func FromError(connectErr *connect.Error) (Error, bool) {
	code, ok := ExtractErrorCode(connectErr)
	if !ok {
		return Error{}, false
	}
	return Lookup(ErrorCode(code))
}

// ExtractErrorCode extracts the domain error code from a *connect.Error.
// The code is read from the google.rpc.ErrorInfo detail, so it works both for
// errors created locally and for errors received from a remote server.
func ExtractErrorCode(connectErr *connect.Error) (string, bool) {
	if connectErr == nil {
		return "", false
	}
	if info, ok := findDetail[*errdetails.ErrorInfo](connectErr); ok && info.GetReason() != "" {
		return info.GetReason(), true
	}
	return "", false
}

// ExtractErrorInfo extracts a google.rpc.ErrorInfo detail from a connect.Error, if present.
func ExtractErrorInfo(err error) (*errdetails.ErrorInfo, bool) {
	connectErr, ok := errors.AsType[*connect.Error](err)
	if !ok {
		return nil, false
	}
	return findDetail[*errdetails.ErrorInfo](connectErr)
}

// ExtractRetryInfo extracts a google.rpc.RetryInfo detail from a connect.Error, if present.
func ExtractRetryInfo(err error) (*errdetails.RetryInfo, bool) {
	connectErr, ok := errors.AsType[*connect.Error](err)
	if !ok {
		return nil, false
	}
	return findDetail[*errdetails.RetryInfo](connectErr)
}

// findDetail returns the first detail of connectErr that decodes to T.
func findDetail[T proto.Message](connectErr *connect.Error) (T, bool) {
	var zero T
	if connectErr == nil {
		return zero, false
	}
	for _, detail := range connectErr.Details() {
		val, err := connectproto.UnmarshalErrorDetail(detail)
		if err != nil {
			continue
		}
		if typed, ok := val.(T); ok {
			return typed, true
		}
	}
	return zero, false
}

// createError is the internal workhorse that all error creation functions delegate to.
// It accepts optional retryDelay (if hasRetryDelay), optional customMsg (if customMsg != ""),
// and an optional domain override (if domainOverride != "")
func createError(code ErrorCoder, data M, retryDelay time.Duration, customMsg string, domainOverride string) *connect.Error {
	codeStr := extractCode(code)
	e, ok := Lookup(ErrorCode(codeStr))
	if !ok {
		connectErr := connect.Errorf(connect.CodeInternal, "unknown error code: %s", codeStr)
		connectErr = withMeta(connectErr, codeStr, false, data, 0, domainOverride)
		getErrorLogger()(codeStr, connect.CodeInternal, false, data)
		return connectErr
	}

	tpl := e.MessageTpl
	if customMsg != "" {
		tpl = customMsg
	}

	if err := ValidateTemplate(tpl, data); err != nil {
		getValidationLogger()(codeStr, data, err)
	}

	msg := FormatTemplate(tpl, data)
	connectErr := connect.NewError(e.StatusCode, msg).WithCause(&CodedError{code: codeStr, msg: msg})

	effectiveDelay := e.RetryDelay
	if retryDelay > 0 {
		effectiveDelay = retryDelay
	}
	connectErr = withMeta(connectErr, codeStr, e.Retryable, data, effectiveDelay, domainOverride)
	getErrorLogger()(codeStr, e.StatusCode, e.Retryable, data)

	return connectErr
}

// New creates a *connect.Error from a registered error code and template data.
// It looks up the error definition in the Registry, formats the message template
// with the provided data, and returns a Connect error with the appropriate status code.
//
// The code parameter must implement ErrorCoder (e.g. ErrorCode or *CodedError).
// If the error code is not found in the Registry, it falls back to CodeInternal.
//
// Example:
//
//	// Using ErrorCode constant
//	return nil, connecterrors.New(connecterrors.ErrNotFound, nil)
//
//	// Using generated error sentinel
//	return nil, connecterrors.New(userv1.ErrUserNotFound, connecterrors.M{"id": "123"})
func New(code ErrorCoder, data M) *connect.Error {
	return createError(code, data, 0, "", "")
}

// NewWithRetry creates a *connect.Error from a registered error code with a custom retry delay.
// This overrides the retry delay defined in the Registry (if any).
// The code parameter must implement ErrorCoder (e.g. ErrorCode or *CodedError).
//
// Example:
//
//	return nil, cerr.NewWithRetry(cerr.ErrUnavailable, cerr.M{}, 5*time.Second)
func NewWithRetry(code ErrorCoder, data M, retryDelay time.Duration) *connect.Error {
	return createError(code, data, retryDelay, "", "")
}

// extractCode extracts the error code string from an ErrorCoder implementation.
func extractCode(code ErrorCoder) string {
	if code == nil {
		return ""
	}
	return code.Code()
}

// NewWithMessage creates a *connect.Error using a custom message template instead of
// the one defined in the Registry. The error code is still used to determine
// the Connect status code and retryable flag.
//
// The code parameter must implement ErrorCoder (e.g. ErrorCode or *CodedError).
//
// Example:
//
//	return nil, connecterrors.NewWithMessage(
//	    connecterrors.ErrNotFound,
//	    "User '{{id}}' does not exist in tenant '{{tenant}}'",
//	    connecterrors.M{"id": "123", "tenant": "acme"},
//	)
func NewWithMessage(code ErrorCoder, customMsg string, data M) *connect.Error {
	return createError(code, data, 0, customMsg, "")
}

// FromCode creates a *connect.Error directly from a Connect status code and message.
// This bypasses the Registry entirely and is useful for one-off errors that don't
// need template support.
//
// Example:
//
//	return nil, connecterrors.FromCode(connect.CodeInternal, "unexpected database error")
func FromCode(code connect.Code, msg string) *connect.Error {
	return connect.NewError(code, msg)
}

// Wrap creates a *connect.Error that wraps an underlying error with context from
// the Registry. The client only sees the template message; the underlying error
// is attached as a local cause, so errors.Is and errors.As still match it on the
// server but its text never reaches the wire. This is useful for adding
// user-facing context to internal errors without leaking them.
//
// The code parameter must implement ErrorCoder (e.g. ErrorCode or *CodedError).
//
// Example:
//
//	user, err := db.GetUser(ctx, id)
//	if err != nil {
//	    return nil, connecterrors.Wrap(connecterrors.ErrNotFound, err, nil)
//	}
func Wrap(code ErrorCoder, err error, data M) *connect.Error {
	codeStr := extractCode(code)
	e, ok := Lookup(ErrorCode(codeStr))
	if !ok {
		connectErr := connect.Errorf(connect.CodeInternal, "unknown error code: %s", codeStr).WithCause(err)
		connectErr = withMeta(connectErr, codeStr, false, data, 0, "")
		getErrorLogger()(codeStr, connect.CodeInternal, false, data)
		return connectErr
	}

	if verr := ValidateTemplate(e.MessageTpl, data); verr != nil {
		getValidationLogger()(codeStr, data, verr)
	}

	msg := FormatTemplate(e.MessageTpl, data)
	connectErr := connect.NewError(e.StatusCode, msg).
		WithCause(&CodedError{code: codeStr, msg: msg}).
		WithCause(err)
	connectErr = withMeta(connectErr, codeStr, e.Retryable, data, e.RetryDelay, "")
	getErrorLogger()(codeStr, e.StatusCode, e.Retryable, data)

	return connectErr
}

// IsRetryableCode checks whether an error code is marked as retryable in the Registry.
// Returns false if the error code is not found.
//
// Example:
//
//	connecterrors.IsRetryableCode(connecterrors.ErrUnavailable) // true
//	connecterrors.IsRetryableCode(connecterrors.ErrNotFound)    // false
func IsRetryableCode(code ErrorCode) bool {
	e, ok := Lookup(code)
	return ok && e.Retryable
}

// IsRetryableErr checks whether an error is marked as retryable.
// Retryable errors carry a google.rpc.RetryInfo detail, so this works both for
// errors created locally and for errors received from a remote server.
// Returns false for non-Connect errors or errors without a RetryInfo detail.
//
// Example:
//
//	err := connecterrors.New(connecterrors.ErrUnavailable, nil)
//	connecterrors.IsRetryableErr(err) // true
func IsRetryableErr(err error) bool {
	_, ok := ExtractRetryInfo(err)
	return ok
}

// IsRetryable checks whether an error code or error is marked as retryable.
//
// Deprecated: Use IsRetryableCode for ErrorCode values or IsRetryableErr for error values.
func IsRetryable(codeOrErr any) bool {
	switch v := codeOrErr.(type) {
	case ErrorCode:
		return IsRetryableCode(v)
	case error:
		return IsRetryableErr(v)
	}
	return false
}

// StatusCode returns the Connect status code for a registered error code.
// Returns connect.CodeInternal if the error code is not found.
func StatusCode(code ErrorCode) connect.Code {
	e, ok := Lookup(code)
	if !ok {
		return connect.CodeInternal
	}
	return e.StatusCode
}

// Newf creates a *connect.Error from a registered error code with a formatted message.
// Instead of using template placeholders, this uses fmt.Sprintf-style formatting.
// The error code is still used to determine the Connect status code and retryable flag.
//
// The code parameter must implement ErrorCoder (e.g. ErrorCode or *CodedError).
//
// Example:
//
//	return nil, cerr.Newf(cerr.ErrNotFound, "User %q not found in org %s", userID, orgName)
func Newf(code ErrorCoder, format string, args ...any) *connect.Error {
	msg := fmt.Sprintf(format, args...)
	return createError(code, nil, 0, msg, "")
}

// CodedError is an error type that carries a domain error code alongside
// the standard error interface. It enables errors.As support
// for matching errors by their registered code.
//
// Example:
//
//	err := cerr.New(cerr.ErrNotFound, nil)
//	var coded *cerr.CodedError
//	if errors.As(err, &coded) {
//	    fmt.Println(coded.Code()) // "ERROR_NOT_FOUND"
//	}
//
// CodedError also implements Is() so that errors.Is works with ErrorCode sentinels:
//
//	errors.Is(err, cerr.ErrNotFound) // true
type CodedError struct {
	code string
	msg  string
}

// Error implements the error interface.
func (e *CodedError) Error() string { return e.msg }

// Code returns the domain error code string.
// This method satisfies the ErrorCoder interface.
func (e *CodedError) Code() string {
	if e == nil {
		return ""
	}
	return e.code
}

// Is reports whether the target is an ErrorCode or *CodedError matching this error's code.
// This enables errors.Is(err, cerr.ErrNotFound) to return true for errors created
// via cerr.New(cerr.ErrNotFound, ...).
func (e *CodedError) Is(target error) bool {
	if e == nil {
		return false
	}
	switch t := target.(type) {
	case ErrorCode:
		return string(t) == e.code
	case *CodedError:
		return t != nil && t.code == e.code
	}
	return false
}

// ErrorCode returns the domain error code (e.g. "ERROR_NOT_FOUND").
// Deprecated: Use Code() instead.
func (e *CodedError) ErrorCode() string { return e.Code() }

// WithDetails returns a copy of connectErr with structured error details appended.
// Connect v2 errors are immutable, so use the returned error.
//
// Example:
//
//	err := cerr.New(cerr.ErrInvalidArgument, nil)
//	detail, _ := connectproto.NewErrorDetail(fieldViolation)
//	err = cerr.WithDetails(err, detail)
func WithDetails(connectErr *connect.Error, details ...*connect.ErrorDetail) *connect.Error {
	for _, d := range details {
		connectErr = connectErr.WithDetail(d)
	}
	return connectErr
}

// WithFieldViolation returns a copy of connectErr with a google.rpc.BadRequest
// FieldViolation detail appended. This is useful for communicating input
// validation failures to clients. Connect v2 errors are immutable, so use the
// returned error.
//
// Example:
//
//	err := cerr.New(cerr.ErrInvalidArgument, nil)
//	err = cerr.WithFieldViolation(err, "email", "must be a valid email address")
func WithFieldViolation(connectErr *connect.Error, field, description string) *connect.Error {
	violation := &errdetails.BadRequest_FieldViolation{
		Field:       field,
		Description: description,
	}
	badRequest := &errdetails.BadRequest{
		FieldViolations: []*errdetails.BadRequest_FieldViolation{violation},
	}
	if detail, err := connectproto.NewErrorDetail(badRequest); err == nil {
		connectErr = connectErr.WithDetail(detail)
	}
	return connectErr
}

// MatchesError reports whether err is a Connect error matching the given error code.
// It checks the google.rpc.ErrorInfo detail, so it works both for errors created
// locally and for errors received from a remote server.
//
// Example:
//
//	if connecterrors.MatchesError(err, connecterrors.ErrNotFound) {
//	    fmt.Println("not found")
//	}
func MatchesError(err error, code ErrorCode) bool {
	connectErr, ok := errors.AsType[*connect.Error](err)
	if !ok {
		return false
	}
	extracted, ok := ExtractErrorCode(connectErr)
	return ok && extracted == string(code)
}

// Matcher pairs an error code with a callback for MatchError.
type Matcher[T any] struct {
	ErrorCode ErrorCode
	Fn        func() T
}

// MatchError is a switch-like error matcher. It tries each matcher in order
// against the error and invokes the first matching callback, returning its result.
// Matchers are evaluated in slice order, making matching deterministic.
// Returns the zero value of T and false if no matcher matches.
//
// Example:
//
//	result, ok := connecterrors.MatchError[string](err, []connecterrors.Matcher[string]{
//	    {ErrorCode: connecterrors.ErrNotFound, Fn: func() string { return "not found" }},
//	    {ErrorCode: connecterrors.ErrInvalidArgument, Fn: func() string { return "bad input" }},
//	})
func MatchError[T any](err error, matchers []Matcher[T]) (T, bool) {
	var zero T
	connectErr, ok := errors.AsType[*connect.Error](err)
	if !ok {
		return zero, false
	}

	for _, m := range matchers {
		if MatchesError(connectErr, m.ErrorCode) {
			return m.Fn(), true
		}
	}

	return zero, false
}
