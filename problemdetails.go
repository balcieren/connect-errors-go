package connecterrors

import (
	"errors"

	"connectrpc.com/connect"
)

// ProblemDetails represents an RFC 7807 (Problem Details for HTTP APIs) error response.
// It is useful for REST/HTTP adapters that need to translate Connect errors to JSON.
type ProblemDetails struct {
	// Type is a URI reference that identifies the problem type.
	// Defaults to "about:blank" if not set.
	Type string `json:"type,omitempty"`

	// Title is a short, human-readable summary of the problem type.
	// It SHOULD NOT change from occurrence to occurrence of the problem.
	Title string `json:"title,omitempty"`

	// Detail is a human-readable explanation specific to this occurrence.
	Detail string `json:"detail,omitempty"`

	// Status is the HTTP status code for this occurrence.
	Status int `json:"status,omitempty"`

	// Instance is a URI reference that identifies the specific occurrence.
	Instance string `json:"instance,omitempty"`
}

// statusCodeToHTTPStatus maps Connect status codes to HTTP status codes.
var statusCodeToHTTPStatus = map[connect.Code]int{
	connect.CodeCanceled:           499,
	connect.CodeUnknown:            500,
	connect.CodeInvalidArgument:    400,
	connect.CodeDeadlineExceeded:   504,
	connect.CodeNotFound:           404,
	connect.CodeAlreadyExists:      409,
	connect.CodePermissionDenied:   403,
	connect.CodeResourceExhausted:  429,
	connect.CodeFailedPrecondition: 412,
	connect.CodeAborted:            409,
	connect.CodeOutOfRange:         400,
	connect.CodeUnimplemented:      501,
	connect.CodeInternal:           500,
	connect.CodeUnavailable:        503,
	connect.CodeDataLoss:           500,
	connect.CodeUnauthenticated:    401,
}

// ToProblemDetails converts a *connect.Error to an RFC 7807 ProblemDetails response.
// It extracts the error code, message, and HTTP status from the Connect error.
// Returns nil if the error is not a *connect.Error.
//
// Example:
//
//	err := cerr.New(cerr.ErrNotFound, nil)
//	pd := cerr.ToProblemDetails(err)
//	// pd.Status = 404, pd.Title = "ERROR_NOT_FOUND", pd.Detail = "Resource not found"
func ToProblemDetails(err error) *ProblemDetails {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return nil
	}

	pd := &ProblemDetails{
		Type:   "about:blank",
		Detail: connectErr.Error(),
		Status: 500,
	}

	if status, ok := statusCodeToHTTPStatus[connectErr.Code()]; ok {
		pd.Status = status
	}

	// Try to extract domain error code for the title
	if code, ok := ExtractErrorCode(connectErr); ok {
		pd.Title = code
	}

	return pd
}
