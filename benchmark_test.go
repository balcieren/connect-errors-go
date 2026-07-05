package connecterrors_test

import (
	"testing"

	connecterrors "github.com/balcieren/connect-errors-go"
)

func BenchmarkFormatTemplate(b *testing.B) {
	tpl := "Resource '{{id}}' not found in {{service}} (tenant: {{tenant}})"
	data := connecterrors.M{"id": "user-123", "service": "auth", "tenant": "acme"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connecterrors.FormatTemplate(tpl, data)
	}
}

func BenchmarkFormatTemplateNoPlaceholders(b *testing.B) {
	tpl := "Internal server error"
	data := connecterrors.M{"id": "123"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connecterrors.FormatTemplate(tpl, data)
	}
}

func BenchmarkLookup(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connecterrors.Lookup(connecterrors.ErrNotFound)
	}
}

func BenchmarkLookupParallel(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			connecterrors.Lookup(connecterrors.ErrNotFound)
		}
	})
}

func BenchmarkErr(b *testing.B) {
	data := connecterrors.M{"id": "user-123"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = connecterrors.New(connecterrors.ErrNotFound, data)
	}
}

func BenchmarkErrParallel(b *testing.B) {
	data := connecterrors.M{"id": "user-123"}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = connecterrors.New(connecterrors.ErrNotFound, data)
		}
	})
}
