package stack_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/themakers/stack"
	"github.com/themakers/stack/stack_backend"
	"github.com/themakers/stack/stack_backend/stack_backend_text"
)

// benchCtx builds a context with a text backend writing to io.Discard, so
// the benchmarks measure the library's work rather than writing to stdout.
func benchCtx() context.Context {
	return stack.With().
		Backend(stack_backend_text.NewWithWriter(io.Discard)).
		ServiceName("bench").
		Environment("test").
		Instance("bench-0").
		Apply(context.Background())
}

// BenchmarkSpanOpenClose — opening and closing a span (the most frequent path).
func BenchmarkSpanOpenClose(b *testing.B) {
	ctx := benchCtx()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, done := stack.Span(ctx)
		done()
	}
}

// BenchmarkInfo_3StringAttrs — a typical log with three string attributes.
func BenchmarkInfo_3StringAttrs(b *testing.B) {
	ctx, done := stack.Span(benchCtx())
	defer done()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stack.Info(ctx, "request handled",
			stack.F("request_url_path", "/api/v1/order"),
			stack.F("method", "POST"),
			stack.F("session", "abc123"),
		)
	}
}

// BenchmarkError — the path that records an error and collects a stack trace.
func BenchmarkError(b *testing.B) {
	ctx, done := stack.Span(benchCtx())
	defer done()
	err := errors.New("boom")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stack.Error(ctx, "operation failed", err,
			stack.F("code", 500),
		)
	}
}

// BenchmarkTextHandle_SpanEnd — rendering a span-end event in the text backend.
func BenchmarkTextHandle_SpanEnd(b *testing.B) {
	backend := stack_backend_text.NewWithWriter(io.Discard)
	e := makeSpanEndEvent()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.Handle(e)
	}
}

// BenchmarkTextHandle_Log — rendering a log event with string attributes.
func BenchmarkTextHandle_Log(b *testing.B) {
	backend := stack_backend_text.NewWithWriter(io.Discard)
	e := makeLogEvent(false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.Handle(e)
	}
}

// BenchmarkTextHandle_LogMapAttr — rendering a log event with a map attribute
// (the cold json.Marshal path).
func BenchmarkTextHandle_LogMapAttr(b *testing.B) {
	backend := stack_backend_text.NewWithWriter(io.Discard)
	e := makeLogEvent(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.Handle(e)
	}
}

// BenchmarkFullPath — the end-to-end path: span+ / info / span-.
func BenchmarkFullPath(b *testing.B) {
	ctx := benchCtx()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, done := stack.Span(ctx)
		stack.Info(c, "processing",
			stack.F("request_url_path", "/api/v1/order"),
			stack.F("user", "u-42"),
			stack.F("session", "abc123"),
		)
		done()
	}
}

func makeSpanEndEvent() stack_backend.Event {
	s := stack_backend.Get(context.Background())
	s.Span.TraceID = stack_backend.NewTraceID()
	s.Span.ID = stack_backend.NewID()
	s.Span.Name = "handler()"
	s.Span.File = "/src/app/handler.go"
	s.Span.Line = 42
	s.Span.Attrs = []stack_backend.Attr{
		{Name: "request_url_path", Value: "/api/v1/order"},
		{Name: "method", Value: "POST"},
		{Name: "session", Value: "abc123"},
		{Name: "user", Value: "u-42"},
	}
	return stack_backend.Event{Kind: stack_backend.KindSpanEnd, State: s}
}

func makeLogEvent(withMap bool) stack_backend.Event {
	s := stack_backend.Get(context.Background())
	s.Span.TraceID = stack_backend.NewTraceID()
	s.Span.ID = stack_backend.NewID()
	s.Span.Attrs = []stack_backend.Attr{
		{Name: "request_url_path", Value: "/api/v1/order"},
		{Name: "method", Value: "POST"},
	}
	own := []stack_backend.Attr{
		{Name: "session", Value: "abc123"},
		{Name: "user", Value: "u-42"},
		{Name: "level", Value: "info"},
	}
	if withMap {
		own = append(own, stack_backend.Attr{
			Name:  "payload",
			Value: map[string]any{"a": 1, "b": "x", "c": true},
		})
	}
	return stack_backend.Event{
		Kind:  stack_backend.KindLog,
		State: s,
		LogEvent: stack_backend.LogEvent{
			ID:       stack_backend.NewID(),
			Name:     "request handled",
			Level:    stack_backend.LevelInfo,
			OwnAttrs: own,
			File:     "/src/app/handler.go",
			Line:     42,
		},
	}
}
