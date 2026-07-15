package stack_backend

import (
	"errors"
	"net/http"
	"strings"
)

// W3C Trace Context.
// traceparent format: "<version>-<trace-id>-<parent-id>-<trace-flags>",
// version="00", trace-id is 16 hex bytes, parent-id is 8 hex bytes.
// See https://www.w3.org/TR/trace-context/

const (
	traceParentHeader = "traceparent"
	traceStateHeader  = "tracestate"
)

var (
	ErrNoTraceContext        = errors.New("stack: no trace context")
	ErrMalformedTraceContext = errors.New("stack: malformed trace context")
)

// ParseW3CTraceParent parses the traceparent header into TraceID and
// ParentSpanID. Returns ErrNoTraceContext for an empty string and
// ErrMalformedTraceContext for a syntactically invalid one.
func ParseW3CTraceParent(traceparent string) (traceID TraceID, parentID ID, err error) {
	if len(traceparent) == 0 {
		return traceID, parentID, ErrNoTraceContext
	}

	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 || parts[0] != "00" {
		return traceID, parentID, ErrMalformedTraceContext
	}

	if traceID, err = TraceIDFromString(parts[1]); err != nil {
		return traceID, parentID, ErrMalformedTraceContext
	}

	if parentID, err = IDFromString(parts[2]); err != nil {
		return traceID, parentID, ErrMalformedTraceContext
	}

	return traceID, parentID, nil
}

// applyW3C is the shared implementation of the W3C options: it applies the
// parsed context to the Stack. A malformed/empty traceparent is ignored (the
// span starts as a root) — in a handler's hot path there is no point failing
// the request over a bad header.
func applyW3C(traceparent string) Option {
	return OptionFunc(func(s *Stack) {
		traceID, parentID, err := ParseW3CTraceParent(traceparent)
		if err != nil {
			return
		}
		s.Span.TraceID = traceID
		s.Span.ParentSpanID = parentID
	})
}

// FormatW3CTraceParent builds the traceparent header from the current span
// state. Returns an empty string when the trace has not started yet (zero
// IDs) — we never emit the invalid "00-000…-000…".
func FormatW3CTraceParent(traceID TraceID, spanID ID) string {
	if traceID.IsZero() || spanID.IsZero() {
		return ""
	}
	return "00-" + traceID.String() + "-" + spanID.String() + "-01"
}

// TraceParentHeaderName / TraceStateHeaderName are the W3C header names.
func TraceParentHeaderName() string { return traceParentHeader }
func TraceStateHeaderName() string  { return traceStateHeader }

// w3cFromRequest extracts traceparent from the request's HTTP headers.
func w3cFromRequest(q *http.Request) Option {
	if q == nil {
		return OptionFunc(func(*Stack) {})
	}
	return applyW3C(q.Header.Get(traceParentHeader))
}
