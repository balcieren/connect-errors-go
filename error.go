package connecterrors

import (
	"errors"
	"fmt"
	"sync/atomic"

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
//
// Example:
//
//	const MyError cerr.ErrorCode = "ERROR_CUSTOM"
//	cerr.New(MyError, cerr.M{"key": "val"})
type ErrorCode string

// Code returns the string representation of the error code.
// This method satisfies the ErrorCoder interface.
func (c ErrorCode) Code() string { return string(c) }

// M is a shorthand type for template data maps.
// Keys are placeholder names, values are their replacements.
//
// Example:
//
//	connecterrors.M{"id": "123", "email": "user@example.com"}
type M map[string]string

// headerKeys holds the configured metadata key names.
type headerKeys struct {
	errorCode string
	retryable string
}

// headerKeysVal stores the current headerKeys atomically for lock-free reads.
var headerKeysVal atomic.Value

func init() {
	headerKeysVal.Store(headerKeys{
		errorCode: "x-error-code",
		retryable: "x-retryable",
	})
}

// getHeaderKeys returns the current header key configuration.
func getHeaderKeys() headerKeys {
	return headerKeysVal.Load().(headerKeys)
}

// SetHeaderKeys reconfigures the metadata keys used for error codes and retryable flags.
// This is safe for concurrent use.
//
// Example:
//
//	cerr.SetHeaderKeys("x-app-error-code", "x-app-retryable")
func SetHeaderKeys(errorCodeKey, retryableKey string) {
	current := getHeaderKeys()
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
func setMeta(connectErr *connect.Error, e Error, data M) {
	hk := getHeaderKeys()
	connectErr.Meta().Set(hk.errorCode, string(e.Code))
	if e.Retryable {
		connectErr.Meta().Set(hk.retryable, "true")
	} else {
		connectErr.Meta().Set(hk.retryable, "false")
	}

	info := &errdetails.ErrorInfo{
		Reason: string(e.Code),
		Domain: "connecterrors",
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

	if e.Retryable {
		retryInfo := &errdetails.RetryInfo{
			RetryDelay: durationpb.New(0),
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

	hk := getHeaderKeys()
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
	hk := getHeaderKeys()
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
//	return nil, connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{"id": "123"})
//
//	// Using generated error sentinel
//	return nil, connecterrors.New(userv1.ErrUserNotFound, connecterrors.M{"id": "123"})
func New(code ErrorCoder, data M) *connect.Error {
	codeStr := extractCode(code)
	e, ok := Lookup(ErrorCode(codeStr))
	if !ok {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("unknown error code: %s", codeStr))
	}

	msg := FormatTemplate(e.MessageTpl, data)
	connectErr := connect.NewError(e.ConnectCode, &CodedError{code: codeStr, msg: msg})
	setMeta(connectErr, e, data)

	return connectErr
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
	codeStr := extractCode(code)
	e, ok := Lookup(ErrorCode(codeStr))
	if !ok {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("unknown error code: %s", codeStr))
	}

	msg := FormatTemplate(customMsg, data)
	connectErr := connect.NewError(e.ConnectCode, &CodedError{code: codeStr, msg: msg})
	setMeta(connectErr, e, data)

	return connectErr
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
//	    return nil, connecterrors.Wrap(connecterrors.ErrNotFound, err, connecterrors.M{"id": id})
//	}
func Wrap(code ErrorCoder, err error, data M) *connect.Error {
	codeStr := extractCode(code)
	e, ok := Lookup(ErrorCode(codeStr))
	if !ok {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("unknown error code %s: %w", codeStr, err))
	}

	msg := FormatTemplate(e.MessageTpl, data)
	wrapped := fmt.Errorf("%w: %w", &CodedError{code: codeStr, msg: msg}, err)
	connectErr := connect.NewError(e.ConnectCode, wrapped)
	setMeta(connectErr, e, data)

	return connectErr
}

// IsRetryable checks whether an error code is marked as retryable in the Registry.
// Returns false if the error code is not found.
func IsRetryable(code ErrorCode) bool {
	e, ok := Lookup(code)
	if !ok {
		return false
	}
	return e.Retryable
}

// ConnectCode returns the Connect status code for a registered error code.
// Returns connect.CodeInternal if the error code is not found.
func ConnectCode(code ErrorCode) connect.Code {
	e, ok := Lookup(code)
	if !ok {
		return connect.CodeInternal
	}
	return e.ConnectCode
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
	codeStr := extractCode(code)
	e, ok := Lookup(ErrorCode(codeStr))
	if !ok {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("unknown error code: %s", codeStr))
	}

	msg := fmt.Sprintf(format, args...)
	connectErr := connect.NewError(e.ConnectCode, &CodedError{code: codeStr, msg: msg})
	setMeta(connectErr, e, nil)

	return connectErr
}

// CodedError is an error type that carries a domain error code alongside
// the standard error interface. It enables errors.Is and errors.As support
// for matching errors by their registered code.
//
// Example:
//
//	err := cerr.New(cerr.ErrNotFound, cerr.M{"id": "123"})
//	var coded *cerr.CodedError
//	if errors.As(err.Unwrap(), &coded) {
//	    fmt.Println(coded.ErrorCode()) // "ERROR_NOT_FOUND"
//	}
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

// ErrorCode returns the domain error code (e.g. "ERROR_NOT_FOUND").
// Deprecated: Use Code() instead.
func (e *CodedError) ErrorCode() string { return e.Code() }

// Is reports whether target is a *CodedError with the same code.
func (e *CodedError) Is(target error) bool {
	t, ok := target.(*CodedError)
	if !ok {
		return false
	}
	return e.code == t.code
}

// NewCodedError creates a sentinel error value for use with errors.Is.
//
// Example:
//
//	var ErrNotFound = cerr.NewCodedError(cerr.ErrNotFound)
//
//	// later:
//	if errors.Is(connectErr.Unwrap(), ErrNotFound) { ... }
func NewCodedError(code ErrorCode) *CodedError {
	return &CodedError{code: string(code)}
}

// WithDetails adds structured error details to an existing *connect.Error.
// Details are protobuf Any messages that can carry domain-specific error information.
// Returns the same error for method chaining.
//
// Example:
//
//	err := cerr.New(cerr.ErrInvalidArgument, cerr.M{"reason": "bad email"})
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
