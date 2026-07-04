package connecterrors

import (
	"context"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
)

// contextExtractorFn is a configurable function that extracts metadata from a context.
// The returned map is merged into the error's template data and ErrorInfo metadata.
var contextExtractorFn atomic.Value

func init() {
	contextExtractorFn.Store(func(context.Context) M { return nil })
}

// SetContextExtractor configures a function that extracts template data from a context.
// The extracted data is merged with the data passed to NewCtx.
// This is useful for automatically injecting trace IDs, request IDs, etc.
//
// Example:
//
//	cerr.SetContextExtractor(func(ctx context.Context) cerr.M {
//	    if traceID, ok := ctx.Value("trace_id").(string); ok {
//	        return cerr.M{"trace_id": traceID}
//	    }
//	    return nil
//	})
func SetContextExtractor(fn func(ctx context.Context) M) {
	if fn != nil {
		contextExtractorFn.Store(fn)
	}
}

// NewCtx creates a *connect.Error from a registered error code, merging template data
// from the context (via SetContextExtractor) with the provided data.
// Context-extracted data is overridden by explicitly provided data.
//
// Example:
//
//	return nil, cerr.NewCtx(ctx, cerr.ErrNotFound, nil)
func NewCtx(ctx context.Context, code ErrorCoder, data M) *connect.Error {
	v := contextExtractorFn.Load()
	if v == nil {
		return New(code, data)
	}
	extractor, ok := v.(func(context.Context) M)
	if !ok {
		return New(code, data)
	}
	ctxData := extractor(ctx)

	merged := make(M, len(ctxData)+len(data))
	for k, v := range ctxData {
		merged[k] = v
	}
	for k, v := range data {
		merged[k] = v
	}

	return New(code, merged)
}

// ErrorBuilder provides a chainable API for constructing Connect errors with
// additional details, retry delays, domain overrides, and field violations.
//
// Example:
//
//	err := cerr.NewBuilder(cerr.ErrInvalidArgument, nil).
//	    WithFieldViolation("email", "already registered").
//	    WithRetryDelay(5 * time.Second).
//	    Build()
type ErrorBuilder struct {
	code         ErrorCoder
	data         M
	retryDelay   time.Duration
	details      []*connect.ErrorDetail
	violations   []fieldViolation
	domain       string
	customMsg    string
}

type fieldViolation struct {
	field       string
	description string
}

// NewBuilder creates a new ErrorBuilder for the given error code and template data.
//
// Example:
//
//	b := cerr.NewBuilder(cerr.ErrNotFound, nil)
func NewBuilder(code ErrorCoder, data M) *ErrorBuilder {
	return &ErrorBuilder{
		code: code,
		data: data,
	}
}

// WithDetail adds a protobuf ErrorDetail to the error.
func (b *ErrorBuilder) WithDetail(d *connect.ErrorDetail) *ErrorBuilder {
	if d != nil {
		b.details = append(b.details, d)
	}
	return b
}

// WithRetryDelay sets a custom retry delay for the error's RetryInfo.
func (b *ErrorBuilder) WithRetryDelay(d time.Duration) *ErrorBuilder {
	b.retryDelay = d
	return b
}

// WithDomain overrides the error domain for this error only.
// This does not affect the global domain configuration.
func (b *ErrorBuilder) WithDomain(domain string) *ErrorBuilder {
	b.domain = domain
	return b
}

// WithFieldViolation adds a google.rpc.BadRequest FieldViolation to the error.
func (b *ErrorBuilder) WithFieldViolation(field, description string) *ErrorBuilder {
	b.violations = append(b.violations, fieldViolation{field, description})
	return b
}

// WithMessage overrides the message template for this error.
func (b *ErrorBuilder) WithMessage(msg string) *ErrorBuilder {
	b.customMsg = msg
	return b
}

// Build constructs the final *connect.Error with all configured options.
func (b *ErrorBuilder) Build() *connect.Error {
	connectErr := createError(b.code, b.data, b.retryDelay, b.customMsg, b.domain)

	// Add field violations
	for _, v := range b.violations {
		WithFieldViolation(connectErr, v.field, v.description)
	}

	// Add extra details
	for _, d := range b.details {
		connectErr.AddDetail(d)
	}

	return connectErr
}
