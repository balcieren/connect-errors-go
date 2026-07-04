package connecterrors

import (
	"context"
	"errors"

	"connectrpc.com/connect"
)

// ErrorInterceptorFunc is a callback invoked when a Connect RPC handler returns
// a *connect.Error that has a registered domain error code (x-error-code metadata).
// It receives the context, the original error, and the resolved Error definition.
//
// Common use cases: logging, metrics, tracing, error transformation.
type ErrorInterceptorFunc func(ctx context.Context, connectErr *connect.Error, def Error)

// ErrorInterceptor is a server-side Connect interceptor that hooks into
// error responses. When a handler returns a *connect.Error with an
// "x-error-code" metadata value, the interceptor resolves it from the
// Registry and invokes the callback.
//
// Example:
//
//	interceptor := cerr.ErrorInterceptor(func(ctx context.Context, err *connect.Error, def cerr.Error) {
//	    slog.ErrorContext(ctx, "rpc error",
//	        "code", def.ErrorCode,
//	        "status_code", def.StatusCode,
//	        "retryable", def.Retryable,
//	    )
//	    metrics.IncrCounter("rpc.error", "code", def.ErrorCode)
//	})
//
//	mux.Handle(userv1connect.NewUserServiceHandler(svc,
//	    connect.WithInterceptors(interceptor),
//	))
func ErrorInterceptor(fn ErrorInterceptorFunc) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err != nil {
				var connectErr *connect.Error
				if ok := asConnectError(err, &connectErr); ok {
					if def, found := FromError(connectErr); found {
						fn(ctx, connectErr, def)
					}
				}
			}
			return resp, err
		}
	}
}

// StreamingHandlerInterceptorFunc is a simple Interceptor implementation that only
// wraps streaming handler RPCs. It has no effect on unary RPCs or streaming clients.
type StreamingHandlerInterceptorFunc func(connect.StreamingHandlerFunc) connect.StreamingHandlerFunc

// WrapUnary implements [connect.Interceptor] by passing through unchanged.
func (s StreamingHandlerInterceptorFunc) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return next
}

// WrapStreamingClient implements [connect.Interceptor] by passing through unchanged.
func (s StreamingHandlerInterceptorFunc) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements [connect.Interceptor] by applying the interceptor function.
func (s StreamingHandlerInterceptorFunc) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return s(next)
}

// StreamingErrorInterceptor is the streaming counterpart of ErrorInterceptor.
// It hooks into streaming RPC error responses and invokes the callback when
// a handler returns a *connect.Error with a registered domain error code.
//
// Example:
//
//	interceptor := cerr.StreamingErrorInterceptor(func(ctx context.Context, err *connect.Error, def cerr.Error) {
//	    slog.ErrorContext(ctx, "streaming rpc error", "code", def.ErrorCode)
//	})
//
//	mux.Handle(userv1connect.NewUserServiceHandler(svc,
//	    connect.WithInterceptors(interceptor),
//	))
func StreamingErrorInterceptor(fn ErrorInterceptorFunc) connect.Interceptor {
	return StreamingHandlerInterceptorFunc(func(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
		return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
			err := next(ctx, conn)
			if err != nil {
				var connectErr *connect.Error
				if ok := asConnectError(err, &connectErr); ok {
					if def, found := FromError(connectErr); found {
						fn(ctx, connectErr, def)
					}
				}
			}
			return err
		}
	})
}

// asConnectError attempts to extract a *connect.Error from err using errors.As,
// which correctly handles wrapped errors.
func asConnectError(err error, target **connect.Error) bool {
	return errors.As(err, target)
}
