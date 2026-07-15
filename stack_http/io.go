package stack_http

import (
	"context"
	"net/http"

	"github.com/themakers/stack/stack_backend"
)

// Errors are kept for compatibility; they delegate to the canonical values in
// stack_backend.
var (
	ErrNoTraceContext         = stack_backend.ErrNoTraceContext
	ErrsMalformedTraceContext = stack_backend.ErrMalformedTraceContext
)

// Import loads the W3C Trace Context from request headers.
func Import(from http.Header) (o stack_backend.Option, err error) {
	return ImportFromString(from.Get(stack_backend.TraceParentHeaderName()), from.Get(stack_backend.TraceStateHeaderName()))
}

// ImportFromString parses traceparent/tracestate into an option. Parsing is
// the single implementation in stack_backend (it used to be duplicated here).
func ImportFromString(traceparent, tracestate string) (o stack_backend.Option, err error) {
	traceID, parentID, err := stack_backend.ParseW3CTraceParent(traceparent)
	if err != nil {
		return nil, err
	}

	// TODO: Flags, TraceState

	return stack_backend.OptionFunc(func(s *stack_backend.Stack) {
		s.Span.TraceID = traceID
		s.Span.ParentSpanID = parentID
	}), nil
}

func Export(from context.Context, to http.Header) {
	var traceparent, tracestate = ExportToString(from)
	if traceparent != "" {
		to.Set(stack_backend.TraceParentHeaderName(), traceparent)
	}

	if tracestate != "" {
		to.Set(stack_backend.TraceStateHeaderName(), tracestate)
	}
}

// ExportToString builds traceparent from the current span. Returns an empty
// string for zero IDs (we never emit the invalid "00-000…-000…-01").
func ExportToString(from context.Context) (traceparent, tracestate string) {
	var s = stack_backend.Get(from)
	return stack_backend.FormatW3CTraceParent(s.Span.TraceID, s.Span.ID), ""
}
