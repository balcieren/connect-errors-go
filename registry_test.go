package connecterrors_test

import (
	"fmt"
	"sync"
	"testing"

	"connectrpc.com/connect"

	connecterrors "github.com/balcieren/connect-errors-go"
)

func TestRegistryDefaultEntries(t *testing.T) {
	codes := []connecterrors.ErrorCode{
		connecterrors.ErrNotFound,
		connecterrors.ErrInvalidArgument,
		connecterrors.ErrAlreadyExists,
		connecterrors.ErrPermissionDenied,
		connecterrors.ErrUnauthenticated,
		connecterrors.ErrInternal,
		connecterrors.ErrUnavailable,
		connecterrors.ErrDeadlineExceeded,
		connecterrors.ErrResourceExhausted,
		connecterrors.ErrFailedPrecondition,
		connecterrors.ErrAborted,
		connecterrors.ErrOutOfRange,
		connecterrors.ErrUnimplemented,
		connecterrors.ErrCanceled,
		connecterrors.ErrDataLoss,
	}

	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			e, ok := connecterrors.Lookup(code)
			if !ok {
				t.Fatalf("Lookup(%q) not found", code)
			}
			if e.ErrorCode != code {
				t.Errorf("ErrorCode = %q, want %q", e.ErrorCode, code)
			}
			if e.MessageTpl == "" {
				t.Errorf("MessageTpl is empty for %q", code)
			}
		})
	}
}

func TestRegistryConnectCodes(t *testing.T) {
	tests := []struct {
		code       connecterrors.ErrorCode
		statusCode connect.Code
	}{
		{connecterrors.ErrNotFound, connect.CodeNotFound},
		{connecterrors.ErrInvalidArgument, connect.CodeInvalidArgument},
		{connecterrors.ErrAlreadyExists, connect.CodeAlreadyExists},
		{connecterrors.ErrPermissionDenied, connect.CodePermissionDenied},
		{connecterrors.ErrUnauthenticated, connect.CodeUnauthenticated},
		{connecterrors.ErrInternal, connect.CodeInternal},
		{connecterrors.ErrUnavailable, connect.CodeUnavailable},
		{connecterrors.ErrDeadlineExceeded, connect.CodeDeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			e, _ := connecterrors.Lookup(tt.code)
			if e.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %v, want %v", e.StatusCode, tt.statusCode)
			}
		})
	}
}

func TestRegistryRetryable(t *testing.T) {
	retryable := []connecterrors.ErrorCode{connecterrors.ErrUnavailable, connecterrors.ErrDeadlineExceeded, connecterrors.ErrResourceExhausted, connecterrors.ErrAborted}
	notRetryable := []connecterrors.ErrorCode{connecterrors.ErrNotFound, connecterrors.ErrInvalidArgument, connecterrors.ErrInternal}

	for _, code := range retryable {
		e, _ := connecterrors.Lookup(code)
		if !e.Retryable {
			t.Errorf("expected %q to be retryable", code)
		}
	}
	for _, code := range notRetryable {
		e, _ := connecterrors.Lookup(code)
		if e.Retryable {
			t.Errorf("expected %q to not be retryable", code)
		}
	}
}

func TestRegister(t *testing.T) {
	connecterrors.Register(connecterrors.Error{
		ErrorCode:  "ERROR_CUSTOM_REG",
		MessageTpl: "Custom: {{detail}}",
		StatusCode: connect.CodeInternal,
	})
	e, ok := connecterrors.Lookup(connecterrors.ErrorCode("ERROR_CUSTOM_REG"))
	if !ok {
		t.Fatal("custom error not found after Register")
	}
	if e.MessageTpl != "Custom: {{detail}}" {
		t.Errorf("MessageTpl = %q", e.MessageTpl)
	}
}

func TestRegisterAll(t *testing.T) {
	connecterrors.RegisterAll([]connecterrors.Error{
		{ErrorCode: "ERROR_B1", MessageTpl: "B1", StatusCode: connect.CodeInternal},
		{ErrorCode: "ERROR_B2", MessageTpl: "B2", StatusCode: connect.CodeNotFound},
	})
	for _, code := range []connecterrors.ErrorCode{"ERROR_B1", "ERROR_B2"} {
		if _, ok := connecterrors.Lookup(code); !ok {
			t.Fatalf("Lookup(%q) not found after RegisterAll", code)
		}
	}
}

func TestLookupNotFound(t *testing.T) {
	if _, ok := connecterrors.Lookup(connecterrors.ErrorCode("ERROR_NONEXISTENT")); ok {
		t.Error("expected not found")
	}
}

func TestMustLookupPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()
	connecterrors.MustLookup(connecterrors.ErrorCode("ERROR_NONEXISTENT_PANIC"))
}

func TestMustLookupSuccess(t *testing.T) {
	e := connecterrors.MustLookup(connecterrors.ErrNotFound)
	if e.ErrorCode != connecterrors.ErrNotFound {
		t.Errorf("ErrorCode = %q", e.ErrorCode)
	}
}

func TestRegisterOverwrite(t *testing.T) {
	connecterrors.Register(connecterrors.Error{ErrorCode: "ERROR_OW", MessageTpl: "Original", StatusCode: connect.CodeInternal})
	connecterrors.Register(connecterrors.Error{ErrorCode: "ERROR_OW", MessageTpl: "Updated", StatusCode: connect.CodeNotFound})
	e, _ := connecterrors.Lookup(connecterrors.ErrorCode("ERROR_OW"))
	if e.MessageTpl != "Updated" {
		t.Errorf("MessageTpl = %q, want Updated", e.MessageTpl)
	}
}

func TestCodes(t *testing.T) {
	codes := connecterrors.Codes()
	if len(codes) < 14 {
		t.Fatalf("expected at least 14 codes, got %d", len(codes))
	}
	// Verify sorted order
	for i := 1; i < len(codes); i++ {
		if codes[i] < codes[i-1] {
			t.Fatalf("codes not sorted: %q < %q", codes[i], codes[i-1])
		}
	}
	// Verify known codes are present
	found := map[string]bool{}
	for _, c := range codes {
		found[c] = true
	}
	for _, want := range []connecterrors.ErrorCode{connecterrors.ErrNotFound, connecterrors.ErrInternal, connecterrors.ErrCanceled} {
		if !found[string(want)] {
			t.Errorf("expected code %q in Codes()", want)
		}
	}
}

func TestConcurrentRegistration(t *testing.T) {
	// This test validates that Register is thread-safe.
	const numGoroutines = 10
	const numOps = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				code := fmt.Sprintf("ERR_CONCURRENT_%d_%d", id, j)
				connecterrors.Register(connecterrors.Error{
					ErrorCode:  connecterrors.ErrorCode(code),
					MessageTpl: "Concurrent error",
					StatusCode: connect.CodeInternal,
				})
			}
		}(i)
	}

	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				_, _ = connecterrors.Lookup(connecterrors.ErrNotFound)
				_ = connecterrors.Codes()
			}
		}()
	}

	wg.Wait()
}
