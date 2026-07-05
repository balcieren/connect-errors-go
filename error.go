package connecterrors

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
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

type headerKeys struct {
	errorCode string
	retryable string
}

// ErrorCodeKey returns the configured error code header key.
func (h headerKeys) ErrorCode() string { return h.errorCode }

// RetryableKey returns the configured retryable header key.
func (h headerKeys) Retryable() string { return h.retryable }

// headerKeysVal stores the current headerKeys atomically for lock-free reads.
var headerKeysVal atomic.Value

// domainVal stores the configured error domain atomically.
var domainVal atomic.Value

func init() {
	headerKeysVal.Store(headerKeys{
		errorCode: "x-error-code",
		retryable: "x-retryable",
	})
	domainVal.Store("connecterrors")
	errorLoggerVal.Store(ErrorLogger(func(code string, statusCode connect.Code, retryable bool, data M) {}))
	validationLoggerVal.Store(ValidationLogger(func(code string, data M, err error) {}))
}

// GetHeaderKeys returns the current header key configuration for error codes and retryable flags.
func GetHeaderKeys() headerKeys {
	return headerKeysVal.Load().(headerKeys)
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

// SetHeaderKeys reconfigures the metadata keys used for error codes and retryable flags.
// This is safe for concurrent use.
//
// Example:
//
//	cerr.SetHeaderKeys("x-app-error-code", "x-app-retryable")
func SetHeaderKeys(errorCodeKey, retryableKey string) {
	writeMu.Lock()
	defer writeMu.Unlock()
	current := GetHeaderKeys()
	if errorCodeKey != "" {
		current.errorCode = errorCodeKey
	}
	if retryableKey != "" {
		current.retryable = retryableKey
	}
	headerKeysVal.Store(current)
}

// setMeta attaches error code and retryable metadata to a Connect error.
// It also attaches google.rpc.ErrorInfo and google.rpc.RetryInfo protobuf details.
// If retryDelay is non-zero, it is used in RetryInfo instead of the default zero delay.
// If domainOverride is non-empty, it is used instead of the global domain.
func setMeta(connectErr *connect.Error, code string, retryable bool, data M, retryDelay time.Duration, domainOverride string) {
	hk := GetHeaderKeys()
	connectErr.Meta().Set(hk.errorCode, code)
	if retryable {
		connectErr.Meta().Set(hk.retryable, "true")
	} else {
		connectErr.Meta().Set(hk.retryable, "false")
	}

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
	if detail, err := connect.NewErrorDetail(info); err == nil {
		connectErr.AddDetail(detail)
	}

	if retryable {
		retryInfo := &errdetails.RetryInfo{
			RetryDelay: durationpb.New(retryDelay),
		}
		if detail, err := connect.NewErrorDetail(retryInfo); err == nil {
			connectErr.AddDetail(detail)
		}
	}
}

// FromError extracts error metadata from a *connect.Error's headers/trailers.
// It reads the configured error code metadata (default "x-error-code") to look up
// the corresponding Error definition in the Registry.
func FromError(connectErr *connect.Error) (Error, bool) {
	if connectErr == nil {
		return Error{}, false
	}

	hk := GetHeaderKeys()
	code := connectErr.Meta().Get(hk.errorCode)
	if code == "" {
		return Error{}, false
	}

	return Lookup(ErrorCode(code))
}

// ExtractErrorCode extracts the domain error code from a *connect.Error's metadata.
func ExtractErrorCode(connectErr *connect.Error) (string, bool) {
	if connectErr == nil {
		return "", false
	}
	hk := GetHeaderKeys()
	code := connectErr.Meta().Get(hk.errorCode)
	if code == "" {
		return "", false
	}
	return code, true
}

// ExtractErrorInfo extracts a google.rpc.ErrorInfo detail from a connect.Error, if present.
func ExtractErrorInfo(err error) (*errdetails.ErrorInfo, bool) {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return nil, false
	}
	for _, detail := range connectErr.Details() {
		val, err := detail.Value()
		if err == nil {
			if info, ok := val.(*errdetails.ErrorInfo); ok {
				return info, true
			}
		}
	}
	return nil, false
}

// ExtractRetryInfo extracts a google.rpc.RetryInfo detail from a connect.Error, if present.
func ExtractRetryInfo(err error) (*errdetails.RetryInfo, bool) {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return nil, false
	}
	for _, detail := range connectErr.Details() {
		val, err := detail.Value()
		if err == nil {
			if info, ok := val.(*errdetails.RetryInfo); ok {
				return info, true
			}
		}
	}
	return nil, false
}

// createError is the internal workhorse that all error creation functions delegate to.
// It accepts optional retryDelay (if hasRetryDelay), optional customMsg (if customMsg != ""),
// and an optional domain override (if domainOverride != "")
func createError(code ErrorCoder, data M, retryDelay time.Duration, customMsg string, domainOverride string) *connect.Error {
	codeStr := extractCode(code)
	e, ok := Lookup(ErrorCode(codeStr))
	if !ok {
		connectErr := connect.NewError(connect.CodeInternal, fmt.Errorf("unknown error code: %s", codeStr))
		setMeta(connectErr, codeStr, false, data, 0, domainOverride)
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
	connectErr := connect.NewError(e.StatusCode, &CodedError{code: codeStr, msg: msg})

	effectiveDelay := e.RetryDelay
	if retryDelay > 0 {
		effectiveDelay = retryDelay
	}
	setMeta(connectErr, codeStr, e.Retryable, data, effectiveDelay, domainOverride)
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
	return connect.NewError(code, errors.New(msg))
}

// Wrap creates a *connect.Error that wraps an underlying error with context from
// the Registry. The original error message is preserved and the template message
// is prepended. This is useful for adding user-facing context to internal errors.
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
		wrapped := fmt.Errorf("unknown error code %s: %w", codeStr, err)
		connectErr := connect.NewError(connect.CodeInternal, wrapped)
		setMeta(connectErr, codeStr, false, data, 0, "")
		getErrorLogger()(codeStr, connect.CodeInternal, false, data)
		return connectErr
	}

	if verr := ValidateTemplate(e.MessageTpl, data); verr != nil {
		getValidationLogger()(codeStr, data, verr)
	}

	msg := FormatTemplate(e.MessageTpl, data)
	wrapped := fmt.Errorf("%w: %w", &CodedError{code: codeStr, msg: msg}, err)
	connectErr := connect.NewError(e.StatusCode, wrapped)
	setMeta(connectErr, codeStr, e.Retryable, data, e.RetryDelay, "")
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

// IsRetryableErr checks whether an error carries retryable metadata.
// It extracts the x-retryable header from the Connect error.
// Returns false for non-Connect errors or errors without retryable metadata.
//
// Example:
//
//	err := connecterrors.New(connecterrors.ErrUnavailable, nil)
//	connecterrors.IsRetryableErr(err) // true
func IsRetryableErr(err error) bool {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return false
	}
	hk := GetHeaderKeys()
	return connectErr.Meta().Get(hk.retryable) == "true"
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
//	if errors.As(err.Unwrap(), &coded) {
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

// WithDetails adds structured error details to an existing *connect.Error.
// Details are protobuf Any messages that can carry domain-specific error information.
// Returns the same error for method chaining.
//
// Example:
//
//	err := cerr.New(cerr.ErrInvalidArgument, nil)
//	detail, _ := connect.NewErrorDetail(fieldViolation)
//	cerr.WithDetails(err, detail)
func WithDetails(connectErr *connect.Error, details ...*connect.ErrorDetail) *connect.Error {
	for _, d := range details {
		if d != nil {
			connectErr.AddDetail(d)
		}
	}
	return connectErr
}

// WithFieldViolation adds a google.rpc.BadRequest with a FieldViolation to a Connect error.
// This is useful for communicating input validation failures to clients.
// Returns the same error for method chaining.
//
// Example:
//
//	err := cerr.New(cerr.ErrInvalidArgument, nil)
//	cerr.WithFieldViolation(err, "email", "must be a valid email address")
func WithFieldViolation(connectErr *connect.Error, field, description string) *connect.Error {
	violation := &errdetails.BadRequest_FieldViolation{
		Field:       field,
		Description: description,
	}
	badRequest := &errdetails.BadRequest{
		FieldViolations: []*errdetails.BadRequest_FieldViolation{violation},
	}
	if detail, err := connect.NewErrorDetail(badRequest); err == nil {
		connectErr.AddDetail(detail)
	}
	return connectErr
}

// MatchesError reports whether err is a Connect error matching the given error code.
// It checks both the error code metadata header and the ErrorInfo protobuf detail,
// making it resilient to transports that drop headers.
//
// Example:
//
//	if connecterrors.MatchesError(err, connecterrors.ErrNotFound) {
//	    fmt.Println("not found")
//	}
func MatchesError(err error, code ErrorCode) bool {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return false
	}

	extracted, ok := ExtractErrorCode(connectErr)
	if ok && extracted == string(code) {
		return true
	}

	if info, ok := ExtractErrorInfo(connectErr); ok {
		return info.Reason == string(code)
	}
	return false
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
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return zero, false
	}

	for _, m := range matchers {
		if MatchesError(connectErr, m.ErrorCode) {
			return m.Fn(), true
		}
	}

	return zero, false
}
