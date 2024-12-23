package stack_http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/themakers/stack/stack_backend"
)

var (
	ErrNoTraceContext         = errors.New("no trace context")
	ErrsMalformedTraceContext = errors.New("malformed trace context")
)

const (
	traceParentHeader = "traceparent"
	traceStateHeader  = "tracestate"
)

// Import loads W3C Trace Context form supplied request header
func Import(from http.Header) (o stack_backend.Option, err error) {
	return ImportFromString(from.Get(traceParentHeader), from.Get(traceStateHeader))
}

func ImportFromString(traceparent, tracestate string) (o stack_backend.Option, err error) {
	if len(traceparent) == 0 {
		return nil, ErrNoTraceContext
	}

	var traceparentSpl = strings.Split(traceparent, "-")
	if len(traceparentSpl) != 4 {
		return nil, ErrsMalformedTraceContext
	}

	if traceparentSpl[0] != "00" {
		return nil, ErrsMalformedTraceContext
	}

	traceID, err := stack_backend.TraceIDFromString(traceparentSpl[1])
	if err != nil {
		return nil, ErrsMalformedTraceContext
	}

	parentID, err := stack_backend.IDFromString(traceparentSpl[2])
	if err != nil {
		return nil, ErrsMalformedTraceContext
	}

	// TODO: Flags

	// TODO: TraceState

	return stack_backend.OptionFunc(func(s *stack_backend.Stack) {
		s.Span.TraceID = traceID
		s.Span.ParentSpanID = parentID
	}), nil
}

func Export(from context.Context, to http.Header) {
	var traceparent, tracestate = ExportToString(from)
	if traceparent != "" {
		to.Set(traceParentHeader, traceparent)
	}

	if tracestate != "" {
		to.Set(traceStateHeader, tracestate)
	}
}

func ExportToString(from context.Context) (traceparent, tracestate string) {
	var s = stack_backend.Get(from)
	return fmt.Sprintf("00-%s-%s-01", s.Span.TraceID.String(), s.Span.ID.String()),
		fmt.Sprintf("")
}
