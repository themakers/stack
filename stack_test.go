package stack_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/themakers/stack"
	"github.com/themakers/stack/stack_backend"
	"github.com/themakers/stack/stack_backend/stack_backend_text"
)

// capture is a test backend collecting events in memory for assertions.
type capture struct {
	mu     sync.Mutex
	events []stack_backend.Event
}

func (c *capture) Handle(e stack_backend.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Copy the Span snapshot: the contract forbids retaining e.State after Handle.
	cp := *e.State
	e.State = &cp
	c.events = append(c.events, e)
}

func (c *capture) Shutdown(context.Context) {}

func (c *capture) spanEnds() []stack_backend.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []stack_backend.Event
	for _, e := range c.events {
		if e.Kind&stack_backend.KindSpanEnd != 0 {
			out = append(out, e)
		}
	}
	return out
}

func newCtx(b stack_backend.Backend) context.Context {
	return stack.With().Backend(b).Apply(context.Background())
}

// Regression: a new span does not inherit the parent's Error (the original
// cdek 401 bug).
func TestSpanDoesNotInheritParentError(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	// A parent with an error.
	pctx, pdone := stack.Span(ctx)
	stack.Error(pctx, "parent failed", errors.New("parent boom"))

	// A child span — must not inherit the error.
	cctx, cdone := stack.Span(pctx)
	cdone()
	pdone()

	ends := cap.spanEnds()
	if len(ends) != 2 {
		t.Fatalf("expected 2 span-end events, got %d", len(ends))
	}

	// The first closed one is the child: there must be no error.
	child := ends[0]
	if child.State.Span.Error != nil {
		t.Fatalf("child span inherited error: %v", child.State.Span.Error)
	}
	_ = cctx
}

// Regression: sibling spans do not clobber each other's Attrs (aliasing in Clone).
func TestSiblingSpansDoNotShareAttrs(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	base, baseDone := stack.Span(ctx, stack.With().Attrs(stack.F("k", "base")))

	_, d1 := stack.Span(base, stack.With().Attrs(stack.F("k", "child1")))
	d1()
	_, d2 := stack.Span(base, stack.With().Attrs(stack.F("k", "child2")))
	d2()
	baseDone()

	ends := cap.spanEnds()
	// Collect the k values from every span-end.
	var vals []string
	for _, e := range ends {
		for _, a := range e.State.Span.Attrs {
			if a.Name == "k" {
				vals = append(vals, a.Value.(string))
			}
		}
	}
	// Expect base|child1, base|child2, base — children must not clobber base.
	joined := strings.Join(vals, ",")
	if !strings.Contains(joined, "child1") || !strings.Contains(joined, "child2") {
		t.Fatalf("missing child attrs: %q", joined)
	}
	// Each child span must contain both base and its own child attr — check
	// that base was not overwritten with child2 in the first child.
	for _, e := range ends {
		var ks []string
		for _, a := range e.State.Span.Attrs {
			if a.Name == "k" {
				ks = append(ks, a.Value.(string))
			}
		}
		// The base span has a single k=base; children have base + childN.
		if len(ks) == 2 && ks[0] != "base" {
			t.Fatalf("child span base attr overwritten: %v", ks)
		}
	}
}

// done(err) marks the span as failed.
func TestDoneWithErrorSetsSpanError(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	_, done := stack.Span(ctx)
	done(errors.New("done boom"))

	ends := cap.spanEnds()
	if len(ends) != 1 {
		t.Fatalf("expected 1 span-end, got %d", len(ends))
	}
	if ends[0].State.Span.Error == nil || ends[0].State.Span.Error.Error() != "done boom" {
		t.Fatalf("done(err) did not set span error: %v", ends[0].State.Span.Error)
	}
}

// done() is idempotent: a repeated call does not send a second span-end.
func TestDoneIdempotent(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	_, done := stack.Span(ctx)
	done()
	done()
	done()

	if n := len(cap.spanEnds()); n != 1 {
		t.Fatalf("expected exactly 1 span-end after repeated done(), got %d", n)
	}
}

// W3C round-trip: importing traceparent → the same trace id is visible in the span.
func TestW3CTraceContextRoundTrip(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	traceID := stack_backend.NewTraceID()
	parentID := stack_backend.NewID()
	tp := stack_backend.FormatW3CTraceParent(traceID, parentID)
	if tp == "" {
		t.Fatal("FormatW3CTraceParent returned empty for non-zero ids")
	}

	_, done := stack.Span(ctx, stack.With().W3CTraceContext(tp, ""))
	done()

	ends := cap.spanEnds()
	if len(ends) != 1 {
		t.Fatalf("expected 1 span-end, got %d", len(ends))
	}
	got := ends[0].State.Span.TraceID
	if got.String() != traceID.String() {
		t.Fatalf("trace id mismatch: got %s want %s", got, traceID)
	}
	if ends[0].State.Span.ParentSpanID.String() != parentID.String() {
		t.Fatalf("parent id mismatch: got %s want %s",
			ends[0].State.Span.ParentSpanID, parentID)
	}
}

func TestW3CMalformedIgnored(t *testing.T) {
	// A malformed traceparent must not fail anything — the span starts as a root.
	_, _, err := stack_backend.ParseW3CTraceParent("garbage")
	if err == nil {
		t.Fatal("expected error for malformed traceparent")
	}
	_, _, err = stack_backend.ParseW3CTraceParent("")
	if !errors.Is(err, stack_backend.ErrNoTraceContext) {
		t.Fatalf("expected ErrNoTraceContext, got %v", err)
	}
}

// The text backend does not panic on "complex" attribute values.
func TestTextBackendNoPanicOnComplexAttrs(t *testing.T) {
	var buf bytes.Buffer
	ctx := stack.With().Backend(stack_backend_text.NewWithWriter(&buf)).Apply(context.Background())

	c, done := stack.Span(ctx)
	ch := make(chan int)
	zero := 0.0
	nan := zero / zero
	stack.Info(c, "complex",
		stack.F("chan", ch),        // unmarshalable value
		stack.F("func", func() {}), // same
		stack.F("map", map[string]int{"a": 1}),
		stack.F("nan", nan), // NaN — json.Marshal returns an error
	)
	done()

	if !strings.Contains(buf.String(), "complex") {
		t.Fatal("expected log line rendered")
	}
	if !strings.Contains(buf.String(), "marshal error") {
		t.Fatal("expected marshal-error placeholder for unmarshalable attrs")
	}
}

// Shutdown reaches nested backends through Tee.
type shutdownSpy struct{ called bool }

func (s *shutdownSpy) Handle(stack_backend.Event) {}
func (s *shutdownSpy) Shutdown(context.Context)   { s.called = true }

func TestTeeShutdownPropagates(t *testing.T) {
	a, b := &shutdownSpy{}, &shutdownSpy{}
	tee := stack_backend.TeeBackend(a, b)
	tee.Shutdown(context.Background())
	if !a.called || !b.called {
		t.Fatalf("shutdown not propagated: a=%v b=%v", a.called, b.called)
	}
}

// Get on an empty context does not panic (noop backend) and logging is safe.
func TestEmptyContextNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic on empty context: %v", r)
		}
	}()
	ctx := context.Background()
	c, done := stack.Span(ctx)
	stack.Info(c, "on empty ctx")
	done()
}

// Concurrent logging into one context — race detector coverage (go test -race).
func TestConcurrentLoggingRace(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)
	rootCtx, done := stack.Span(ctx, stack.With().EmbedLogsIntoSpans(true))
	defer done()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				stack.Info(rootCtx, "concurrent", stack.F("g", n), stack.F("j", j))
				cctx, cdone := stack.Span(rootCtx)
				stack.Warn(cctx, "child log")
				cdone()
			}
		}(i)
	}
	wg.Wait()
}

// Cyrillic (valid UTF-8) is rendered raw, without json.Marshal \u-escapes.
func TestTextBackendUTF8Raw(t *testing.T) {
	var buf bytes.Buffer
	ctx := stack.With().Backend(stack_backend_text.NewWithWriter(&buf)).Apply(context.Background())

	c, done := stack.Span(ctx)
	stack.Info(c, "проверка",
		stack.F("город", "Санкт-Петербург"),
		stack.F("quoted", `has "quotes"`),
		stack.F("broken", string([]byte{0xff, 0xfe, 'a'})),
	)
	done()

	out := buf.String()
	if !strings.Contains(out, `город="Санкт-Петербург"`) {
		t.Fatalf("cyrillic not rendered raw: %s", out)
	}
	if !strings.Contains(out, `quoted="has \"quotes\""`) {
		t.Fatalf("quotes not escaped: %s", out)
	}
	if strings.Contains(out, "\xff") {
		t.Fatalf("invalid utf-8 leaked raw: %q", out)
	}
}

// TLog does not panic on non-structs.
func TestTLogNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("TLog panicked: %v", r)
		}
	}()
	var buf bytes.Buffer
	ctx := stack.With().Backend(stack_backend_text.NewWithWriter(&buf)).Apply(context.Background())

	stack.TLog(ctx, 42)                      // not a struct
	stack.TLog(ctx, nil)                     // nil
	stack.TLog(ctx, (*struct{ X int })(nil)) // nil pointer
	stack.TLog(ctx, struct{ X int }{X: 7})   // valid input

	if !strings.Contains(buf.String(), "TLog") {
		t.Fatalf("expected warn logs for invalid TLog input, got: %s", buf.String())
	}
}

// done() while other goroutines keep logging into the same ctx — no races
// (span-end hands the backend a snapshot, not the live *Stack).
func TestDoneDuringConcurrentLogging(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	sctx, done := stack.Span(ctx, stack.With().EmbedLogsIntoSpans(true))

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; ; j++ {
				select {
				case <-stop:
					return
				default:
					stack.Info(sctx, "during done", stack.F("g", n), stack.F("j", j))
				}
			}
		}(i)
	}

	done() // close the span while goroutines keep logging
	close(stop)
	wg.Wait()
}

// The ID generator yields non-zero unique values.
func TestIDGeneration(t *testing.T) {
	seen := map[string]bool{}
	for i := range 1000 {
		id := stack_backend.NewTraceID()
		if id.IsZero() {
			t.Fatal("generated zero trace id")
		}
		if seen[id.String()] {
			t.Fatalf("duplicate trace id at iter %d", i)
		}
		seen[id.String()] = true
	}
}
