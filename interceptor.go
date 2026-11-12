package connecterrors

import (
	"context"
	"errors"

	"connectrpc.com/connect/v2"
)

// ErrorInterceptorFunc is a callback invoked when a Connect RPC handler returns
// a *connect.Error that carries a registered domain error code.
// It receives the context, the original error, and the resolved Error definition.
//
// Common use cases: logging, metrics, tracing, error transformation.
type ErrorInterceptorFunc func(ctx context.Context, connectErr *connect.Error, def Error)

// ErrorInterceptor is a server-side Connect interceptor that hooks into
// error responses. When a handler returns a *connect.Error whose domain error
// code is found in the Registry, the interceptor invokes the callback.
//
// Connect v2 server interceptors wrap every RPC type, so the same interceptor
// covers unary and streaming handlers.
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
//	server := connect.NewServer(interceptor)
//	userv1connect.RegisterUserServiceHandler(server, svc)
//	connecthttp.Mount(mux, server)
func ErrorInterceptor(fn ErrorInterceptorFunc) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			err := next(ctx, spec, stream)
			if err != nil {
				if connectErr, ok := errors.AsType[*connect.Error](err); ok {
					if def, found := FromError(connectErr); found {
						fn(ctx, connectErr, def)
					}
				}
			}
			return err
		}
	}
}
