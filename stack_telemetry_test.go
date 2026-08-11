package stack_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/themakers/stack"
	"github.com/themakers/stack/stack_backend"
)

// logs returns the captured log events (spanEnds' counterpart from stack_test.go).
func (c *capture) logs() []stack_backend.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []stack_backend.Event
	for _, e := range c.events {
		if e.Kind&stack_backend.KindLog != 0 {
			out = append(out, e)
		}
	}
	return out
}

// Transient reports an error but must not fail the span: a retried or
// fallen-back-from error is not a failed operation.
func TestTransientDoesNotFailSpan(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	sctx, done := stack.Span(ctx)
	stack.Transient(sctx, "attempt failed, retrying", errors.New("connection reset"))
	done()

	ends := cap.spanEnds()
	if len(ends) != 1 {
		t.Fatalf("expected 1 span-end event, got %d", len(ends))
	}
	if err := ends[0].State.Span.Error; err != nil {
		t.Fatalf("span must not be failed by Transient, got %v", err)
	}
}

// The error itself must survive in full: warn level, the error payload and a
// stack trace — otherwise a handled error becomes undebuggable.
func TestTransientKeepsErrorFidelity(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	cause := errors.New("connection reset")
	returned := stack.Transient(ctx, "attempt failed", cause)

	if !errors.Is(returned, cause) {
		t.Fatalf("returned error must wrap the cause, got %v", returned)
	}

	logs := cap.logs()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log event, got %d", len(logs))
	}
	e := logs[0]
	if e.LogEvent.Level != stack_backend.LevelWarn {
		t.Fatalf("expected warn level, got %q", e.LogEvent.Level)
	}
	if e.LogEvent.Error == nil {
		t.Fatal("the log event must carry the error")
	}
	if e.LogEvent.StackTrace == nil {
		t.Fatal("the log event must carry a stack trace")
	}
	// KindError drives backend routing, so a handled error must not raise it.
	if e.Kind&stack_backend.KindError != 0 {
		t.Fatal("Transient must not set KindError")
	}
}

// Error keeps its contract: it does fail the span and does raise KindError.
func TestErrorStillFailsSpan(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	sctx, done := stack.Span(ctx)
	stack.Error(sctx, "operation failed", errors.New("boom"))
	done()

	ends := cap.spanEnds()
	if len(ends) != 1 {
		t.Fatalf("expected 1 span-end event, got %d", len(ends))
	}
	if ends[0].State.Span.Error == nil {
		t.Fatal("Error must fail the span")
	}

	logs := cap.logs()
	if len(logs) != 1 || logs[0].Kind&stack_backend.KindError == 0 {
		t.Fatal("Error must set KindError")
	}
}

// A Transient before an Error must not shadow the real failure.
func TestTransientThenErrorFailsSpan(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	sctx, done := stack.Span(ctx)
	stack.Transient(sctx, "attempt 1 failed", errors.New("timeout"))
	stack.Error(sctx, "gave up", errors.New("boom"))
	done()

	ends := cap.spanEnds()
	if len(ends) != 1 {
		t.Fatalf("expected 1 span-end event, got %d", len(ends))
	}
	if err := ends[0].State.Span.Error; err == nil {
		t.Fatal("the span must be failed by the trailing Error")
	} else if err.Error() != "boom" {
		t.Fatalf("expected the Error cause to win, got %q", err)
	}
}

func TestSpanKind(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	_, done := stack.Span(ctx, stack.With().Kind(stack_backend.SpanKindServer))
	done()

	if kind := cap.spanEnds()[0].State.Span.Kind; kind != stack_backend.SpanKindServer {
		t.Fatalf("expected server kind, got %v", kind)
	}
}

// A child of a SERVER span is not itself a server.
func TestSpanKindNotInherited(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	pctx, pdone := stack.Span(ctx, stack.With().Kind(stack_backend.SpanKindServer))
	_, cdone := stack.Span(pctx)
	cdone()
	pdone()

	ends := cap.spanEnds()
	if kind := ends[0].State.Span.Kind; kind != stack_backend.SpanKindInternal {
		t.Fatalf("the child must default to internal, got %v", kind)
	}
}

// NewTrace makes the span a root of its own trace — the mechanism behind
// per-iteration traces in long-lived loops.
func TestNewTrace(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	pctx, pdone := stack.Span(ctx)
	parentTrace := stack_backend.Get(pctx).Span.TraceID

	tctx, tdone := stack.Span(pctx, stack.With().NewTrace())
	tickTrace := stack_backend.Get(tctx).Span.TraceID
	tickSpan := stack_backend.Get(tctx).Span
	tdone()
	pdone()

	if tickTrace == parentTrace {
		t.Fatal("NewTrace must start a different trace")
	}
	if tickTrace.IsZero() {
		t.Fatal("NewTrace must still generate a trace id")
	}
	if tickSpan.ID.IsZero() {
		t.Fatal("NewTrace must still generate a span id")
	}
	if !tickSpan.ParentSpanID.IsZero() {
		t.Fatal("a new trace's root span must have no parent")
	}
}

// Children of a NewTrace span join the new trace, not the old one.
func TestNewTraceChildInherits(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	pctx, pdone := stack.Span(ctx)
	tctx, tdone := stack.Span(pctx, stack.With().NewTrace())
	tickTrace := stack_backend.Get(tctx).Span.TraceID
	tickID := stack_backend.Get(tctx).Span.ID

	cctx, cdone := stack.Span(tctx)
	child := stack_backend.Get(cctx).Span

	cdone()
	tdone()
	pdone()

	if child.TraceID != tickTrace {
		t.Fatal("a child must stay in the tick's trace")
	}
	if child.ParentSpanID != tickID {
		t.Fatal("a child must point at the tick span as its parent")
	}
}

func TestLink(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	upstreamTrace := stack_backend.NewTraceID()
	upstreamSpan := stack_backend.NewID()

	_, done := stack.Span(ctx, stack.With().Link(
		upstreamTrace, upstreamSpan, stack.F("reason", "enqueued_by"),
	))
	done()

	links := cap.spanEnds()[0].State.Span.Links
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].TraceID != upstreamTrace || links[0].SpanID != upstreamSpan {
		t.Fatal("the link must carry the upstream ids")
	}
	if len(links[0].Attrs) != 1 || links[0].Attrs[0].Name != "reason" {
		t.Fatal("the link must carry its attributes")
	}
}

// A zero id means "no upstream context": a broken link is worse than none.
func TestLinkZeroIDsIgnored(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	_, done := stack.Span(ctx, stack.With().
		Link(stack_backend.TraceID{}, stack_backend.NewID()).
		Link(stack_backend.NewTraceID(), stack_backend.ID{}))
	done()

	if links := cap.spanEnds()[0].State.Span.Links; len(links) != 0 {
		t.Fatalf("zero ids must be ignored, got %d links", len(links))
	}
}

func TestLinkTraceparent(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	traceID := stack_backend.NewTraceID()
	spanID := stack_backend.NewID()
	tp := stack_backend.FormatW3CTraceParent(traceID, spanID)

	_, done := stack.Span(ctx, stack.With().LinkTraceparent(tp))
	done()

	links := cap.spanEnds()[0].State.Span.Links
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].TraceID != traceID || links[0].SpanID != spanID {
		t.Fatal("the link must decode the traceparent")
	}
}

func TestLinkTraceparentMalformedIgnored(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	_, done := stack.Span(ctx, stack.With().
		LinkTraceparent("").
		LinkTraceparent("garbage").
		LinkTraceparent("00-tooshort-x-01"))
	done()

	if links := cap.spanEnds()[0].State.Span.Links; len(links) != 0 {
		t.Fatalf("malformed traceparents must be ignored, got %d links", len(links))
	}
}

// Links describe the parent's causality — a child must not repeat them.
func TestLinksNotInheritedByChild(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	pctx, pdone := stack.Span(ctx, stack.With().Link(
		stack_backend.NewTraceID(), stack_backend.NewID(),
	))
	_, cdone := stack.Span(pctx)
	cdone()
	pdone()

	ends := cap.spanEnds()
	if links := ends[0].State.Span.Links; len(links) != 0 {
		t.Fatalf("the child must not inherit links, got %d", len(links))
	}
	if links := ends[1].State.Span.Links; len(links) != 1 {
		t.Fatalf("the parent must keep its link, got %d", len(links))
	}
}

func TestExportTraceparent(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	sctx, done := stack.Span(ctx)
	defer done()

	span := stack_backend.Get(sctx).Span

	h := http.Header{}
	stack.ExportTraceparent(sctx, h)

	got := h.Get("traceparent")
	want := stack_backend.FormatW3CTraceParent(span.TraceID, span.ID)
	if got != want {
		t.Fatalf("traceparent mismatch: got %q, want %q", got, want)
	}

	// Round trip: the receiving side must recover the same ids.
	traceID, parentID, err := stack_backend.ParseW3CTraceParent(got)
	if err != nil {
		t.Fatalf("the exported traceparent must parse back: %v", err)
	}
	if traceID != span.TraceID || parentID != span.ID {
		t.Fatal("the round trip must preserve the ids")
	}
}

// Without a span there is nothing valid to export: an all-zero traceparent
// would make the callee start a broken trace.
func TestExportTraceparentNoSpan(t *testing.T) {
	h := http.Header{}
	stack.ExportTraceparent(context.Background(), h)

	if got := h.Get("traceparent"); got != "" {
		t.Fatalf("expected no header without a span, got %q", got)
	}
}

func TestExportTraceparentNilHeader(t *testing.T) {
	// Must not panic.
	stack.ExportTraceparent(context.Background(), nil)
}

func TestTraceContext(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	sctx, done := stack.Span(ctx)
	defer done()

	span := stack_backend.Get(sctx).Span
	traceID, spanID := stack.TraceContext(sctx)

	if traceID != span.TraceID || spanID != span.ID {
		t.Fatal("TraceContext must return the current span's ids")
	}

	if tid, sid := stack.TraceContext(context.Background()); !tid.IsZero() || !sid.IsZero() {
		t.Fatal("TraceContext must be zero without a span")
	}
}

func TestWithHostFields(t *testing.T) {
	cap := &capture{}
	ctx := stack.With().Backend(cap).Option(stack.WithHostFields()).Apply(context.Background())

	_, done := stack.Span(ctx)
	done()

	host, err := os.Hostname()
	if err != nil || host == "" {
		t.Skip("no hostname available in this environment")
	}

	var found string
	for _, a := range cap.spanEnds()[0].State.Options.ScopeAttrs {
		if a.Name == "host.name" {
			found = a.Value.String()
		}
	}
	if found != host {
		t.Fatalf("host.name mismatch: got %q, want %q", found, host)
	}
}
