package connecterrors_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	connecterrors "github.com/balcieren/connect-errors-go"
)

func TestErrorInterceptor(t *testing.T) {
	var captured connecterrors.Error
	var capturedErr *connect.Error

	interceptor := connecterrors.ErrorInterceptor(func(_ context.Context, err *connect.Error, def connecterrors.Error) {
		capturedErr = err
		captured = def
	})

	// Simulate a handler that returns a domain error
	domainErr := connecterrors.New(connecterrors.ErrNotFound, nil)

	handler := interceptor(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, domainErr
	})

	_, err := handler(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error to be returned")
	}

	if capturedErr == nil {
		t.Fatal("expected interceptor callback to be invoked")
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

	// Raw connect.Error without x-error-code metadata
	rawErr := connect.NewError(connect.CodeInternal, nil)
	handler := interceptor(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, rawErr
	})

	_, _ = handler(context.Background(), nil)
	if called {
		t.Error("interceptor should not be called for errors without x-error-code metadata")
	}
}

func TestErrorInterceptorNoError(t *testing.T) {
	called := false
	interceptor := connecterrors.ErrorInterceptor(func(_ context.Context, _ *connect.Error, _ connecterrors.Error) {
		called = true
	})

	handler := interceptor(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, nil
	})

	_, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatal("expected no error")
	}
	if called {
		t.Error("interceptor should not be called when there is no error")
	}
}

// mockStreamingHandlerConn is a minimal StreamingHandlerConn for testing.
type mockStreamingHandlerConn struct {
	connect.StreamingHandlerConn
}

func TestStreamingErrorInterceptor(t *testing.T) {
	var captured connecterrors.Error

	interceptor := connecterrors.StreamingErrorInterceptor(func(_ context.Context, _ *connect.Error, def connecterrors.Error) {
		captured = def
	})

	domainErr := connecterrors.New(connecterrors.ErrNotFound, nil)

	// Cast to get the underlying StreamingHandlerInterceptorFunc and call it
	wrappedHandler := interceptor.WrapStreamingHandler(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		return domainErr
	})

	err := wrappedHandler(context.Background(), &mockStreamingHandlerConn{})
	if err == nil {
		t.Fatal("expected error to be returned")
	}

	if captured.ErrorCode != connecterrors.ErrNotFound {
		t.Errorf("captured code = %q, want %q", captured.ErrorCode, connecterrors.ErrNotFound)
	}
}

func TestStreamingErrorInterceptorNoMeta(t *testing.T) {
	called := false
	interceptor := connecterrors.StreamingErrorInterceptor(func(_ context.Context, _ *connect.Error, _ connecterrors.Error) {
		called = true
	})

	rawErr := connect.NewError(connect.CodeInternal, nil)
	wrappedHandler := interceptor.WrapStreamingHandler(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		return rawErr
	})

	_ = wrappedHandler(context.Background(), &mockStreamingHandlerConn{})
	if called {
		t.Error("streaming interceptor should not be called for errors without x-error-code metadata")
	}
}

func TestStreamingErrorInterceptorNoError(t *testing.T) {
	called := false
	interceptor := connecterrors.StreamingErrorInterceptor(func(_ context.Context, _ *connect.Error, _ connecterrors.Error) {
		called = true
	})

	wrappedHandler := interceptor.WrapStreamingHandler(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		return nil
	})

	err := wrappedHandler(context.Background(), &mockStreamingHandlerConn{})
	if err != nil {
		t.Fatal("expected no error")
	}
	if called {
		t.Error("streaming interceptor should not be called when there is no error")
	}
}
