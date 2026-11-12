package connecterrors_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"

	connecterrors "github.com/balcieren/connect-errors-go/v2"
)

var unarySpec = connect.Spec{Procedure: "/test.v1.TestService/Unary", StreamType: connect.StreamTypeUnary}

var bidiSpec = connect.Spec{Procedure: "/test.v1.TestService/Bidi", StreamType: connect.StreamTypeBidi}

func TestErrorInterceptor(t *testing.T) {
	var captured connecterrors.Error
	var capturedErr *connect.Error

	interceptor := connecterrors.ErrorInterceptor(func(_ context.Context, err *connect.Error, def connecterrors.Error) {
		capturedErr = err
		captured = def
	})

	// Simulate a handler that returns a domain error
	domainErr := connecterrors.New(connecterrors.ErrNotFound, nil)

	handler := interceptor(func(context.Context, connect.Spec, connect.ServerStream) error {
		return domainErr
	})

	if err := handler(context.Background(), unarySpec, nil); err == nil {
		t.Fatal("expected error to be returned")
	}

	if capturedErr == nil {
		t.Fatal("expected interceptor callback to be invoked")
	}
	if captured.ErrorCode != connecterrors.ErrNotFound {
		t.Errorf("captured code = %q, want %q", captured.ErrorCode, connecterrors.ErrNotFound)
	}
}

func TestErrorInterceptorStreaming(t *testing.T) {
	var captured connecterrors.Error

	interceptor := connecterrors.ErrorInterceptor(func(_ context.Context, _ *connect.Error, def connecterrors.Error) {
		captured = def
	})

	domainErr := connecterrors.New(connecterrors.ErrNotFound, nil)
	handler := interceptor(func(context.Context, connect.Spec, connect.ServerStream) error {
		return domainErr
	})

	if err := handler(context.Background(), bidiSpec, nil); err == nil {
		t.Fatal("expected error to be returned")
	}
	if captured.ErrorCode != connecterrors.ErrNotFound {
		t.Errorf("captured code = %q, want %q", captured.ErrorCode, connecterrors.ErrNotFound)
	}
}

func TestErrorInterceptorNoMeta(t *testing.T) {
	called := false
	interceptor := connecterrors.ErrorInterceptor(func(_ context.Context, _ *connect.Error, _ connecterrors.Error) {
		called = true
	})

	// Raw connect.Error without a domain error code
	rawErr := connect.NewError(connect.CodeInternal, "")
	handler := interceptor(func(context.Context, connect.Spec, connect.ServerStream) error {
		return rawErr
	})

	_ = handler(context.Background(), unarySpec, nil)
	if called {
		t.Error("interceptor should not be called for errors without a domain error code")
	}
}

func TestErrorInterceptorNoError(t *testing.T) {
	called := false
	interceptor := connecterrors.ErrorInterceptor(func(_ context.Context, _ *connect.Error, _ connecterrors.Error) {
		called = true
	})

	handler := interceptor(func(context.Context, connect.Spec, connect.ServerStream) error {
		return nil
	})

	if err := handler(context.Background(), unarySpec, nil); err != nil {
		t.Fatal("expected no error")
	}
	if called {
		t.Error("interceptor should not be called when there is no error")
	}
}
