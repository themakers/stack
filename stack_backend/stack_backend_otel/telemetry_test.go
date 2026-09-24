package stack_backend_otel_test

import (
	"context"
	"errors"
	"testing"
	"time"

	trace_model_v1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/themakers/stack"
	"github.com/themakers/stack/stack_backend"
	"github.com/themakers/stack/stack_backend/stack_backend_otel"
)

func firstSpan(t *testing.T, sink *fakeSink) *trace_model_v1.Span {
	t.Helper()
	for _, rs := range sink.traces {
		for _, rss := range rs.ResourceSpans {
			for _, ss := range rss.ScopeSpans {
				for _, s := range ss.Spans {
					return s
				}
			}
		}
	}
	t.Fatal("no span exported")
	return nil
}

// The kind must reach OTLP; the default is INTERNAL, never UNSPECIFIED — the
// spec tells consumers to assume INTERNAL anyway, and being explicit lets a
// reader tell "internal" from "the producer forgot to set it".
func TestSpanKindExported(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind stack_backend.SpanKind
		want trace_model_v1.Span_SpanKind
	}{
		{"default", stack_backend.SpanKindInternal, trace_model_v1.Span_SPAN_KIND_INTERNAL},
		{"server", stack_backend.SpanKindServer, trace_model_v1.Span_SPAN_KIND_SERVER},
		{"client", stack_backend.SpanKindClient, trace_model_v1.Span_SPAN_KIND_CLIENT},
		{"producer", stack_backend.SpanKindProducer, trace_model_v1.Span_SPAN_KIND_PRODUCER},
		{"consumer", stack_backend.SpanKindConsumer, trace_model_v1.Span_SPAN_KIND_CONSUMER},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &fakeSink{}
			ctx := stack.With().Backend(stack_backend_otel.New(sink)).Apply(context.Background())

			_, done := stack.Span(ctx, stack.With().Kind(tc.kind))
			done()

			if got := firstSpan(t, sink).Kind; got != tc.want {
				t.Fatalf("kind mismatch: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSpanLinksExported(t *testing.T) {
	sink := &fakeSink{}
	ctx := stack.With().Backend(stack_backend_otel.New(sink)).Apply(context.Background())

	upstreamTrace := stack_backend.NewTraceID()
	upstreamSpan := stack_backend.NewID()

	_, done := stack.Span(ctx, stack.With().Link(
		upstreamTrace, upstreamSpan, stack.F("reason", "enqueued_by"),
	))
	done()

	span := firstSpan(t, sink)
	if len(span.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(span.Links))
	}
	link := span.Links[0]
	if string(link.TraceId) != string(upstreamTrace.Bytes()) {
		t.Fatal("link trace id mismatch")
	}
	if string(link.SpanId) != string(upstreamSpan.Bytes()) {
		t.Fatal("link span id mismatch")
	}
	if len(link.Attributes) != 1 || link.Attributes[0].Key != "reason" {
		t.Fatal("the link must carry its attributes")
	}
}

// A handled error must not paint the exported span red.
func TestTransientLeavesSpanStatusUnset(t *testing.T) {
	sink := &fakeSink{}
	ctx := stack.With().Backend(stack_backend_otel.New(sink)).Apply(context.Background())

	c, done := stack.Span(ctx)
	stack.Transient(c, "retrying", errors.New("timeout"))
	done()

	span := firstSpan(t, sink)
	if span.Status != nil && span.Status.Code == trace_model_v1.Status_STATUS_CODE_ERROR {
		t.Fatal("Transient must not set the span status to ERROR")
	}
}

// The counterpart of the previous test: a real failure does paint the span.
func TestErrorSetsSpanStatus(t *testing.T) {
	sink := &fakeSink{}
	ctx := stack.With().Backend(stack_backend_otel.New(sink)).Apply(context.Background())

	c, done := stack.Span(ctx)
	stack.Error(c, "rpc failed", errors.New("boom"))
	done()

	span := firstSpan(t, sink)
	if span.Status == nil || span.Status.Code != trace_model_v1.Status_STATUS_CODE_ERROR {
		t.Fatal("Error must set the span status to ERROR")
	}
}

func TestHostNameInResource(t *testing.T) {
	sink := &fakeSink{}
	ctx := stack.With().
		Backend(stack_backend_otel.New(sink)).
		Option(stack.WithHostFields()).
		Apply(context.Background())

	_, done := stack.Span(ctx)
	done()

	var found bool
	for _, rss := range sink.traces[0].ResourceSpans {
		for _, kv := range rss.Resource.Attributes {
			if kv.Key == "host.name" && kv.Value.GetStringValue() != "" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("host.name must be exported as a resource attribute")
	}
}

// A span recorded after the fact (imported from a client) keeps its own ids,
// times and resource; its child and a LogAt record inside it keep theirs, and
// the importing span is not failed by an imported error record.
func TestImportedSpanAndLogAt(t *testing.T) {
	sink := &fakeSink{}
	ctx := stack.With().Backend(stack_backend_otel.New(sink)).ServiceName("server").Apply(context.Background())

	reqCtx, reqDone := stack.Span(ctx, stack.With().Name("request"))

	traceID := stack_backend.NewTraceID()
	spanID := stack_backend.NewID()
	parentID := stack_backend.NewID()
	start := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	end := start.Add(1500 * time.Millisecond)
	logAt := start.Add(700 * time.Millisecond)

	cctx, done := stack.Span(reqCtx, stack.With().
		Name("client.join").
		ServiceName("web").
		TraceID(traceID.Bytes()).
		ParentSpanID(parentID.Bytes()).
		SpanID(spanID.Bytes()).
		StartTime(start).
		EndTime(end))
	stack.LogAt(cctx, logAt, stack_backend.LevelError, "client error", errors.New("boom"))
	//> A child of an imported span does not inherit its fixed end.
	_, childDone := stack.Span(cctx, stack.With().Name("child"))
	childDone()
	done()
	reqDone()

	spans := map[string]*trace_model_v1.Span{}
	services := map[string]string{}
	for _, td := range sink.traces {
		for _, rs := range td.ResourceSpans {
			svc := ""
			for _, a := range rs.Resource.Attributes {
				if a.Key == "service.name" {
					svc = a.Value.GetStringValue()
				}
			}
			for _, ss := range rs.ScopeSpans {
				for _, s := range ss.Spans {
					spans[s.Name] = s
					services[s.Name] = svc
				}
			}
		}
	}
	imp := spans["client.join"]
	if imp == nil {
		t.Fatalf("imported span not exported: %v", spans)
	}
	if string(imp.TraceId) != string(traceID.Bytes()) || string(imp.SpanId) != string(spanID.Bytes()) ||
		string(imp.ParentSpanId) != string(parentID.Bytes()) {
		t.Fatal("imported span identity changed")
	}
	if imp.StartTimeUnixNano != uint64(start.UnixNano()) || imp.EndTimeUnixNano != uint64(end.UnixNano()) {
		t.Fatalf("imported span times: %d..%d", imp.StartTimeUnixNano, imp.EndTimeUnixNano)
	}
	if services["client.join"] != "web" || services["request"] != "server" {
		t.Fatalf("resources: %v", services)
	}
	child := spans["child"]
	if child == nil || string(child.ParentSpanId) != string(spanID.Bytes()) || child.EndTimeUnixNano <= uint64(end.UnixNano()) {
		t.Fatalf("child of imported span: %+v", child)
	}
	if req := spans["request"]; req.Status != nil && req.Status.Code == trace_model_v1.Status_STATUS_CODE_ERROR {
		t.Fatal("LogAt must not fail the importing span")
	}

	var found bool
	for _, ld := range sink.logs {
		for _, rl := range ld.ResourceLogs {
			for _, sl := range rl.ScopeLogs {
				for _, r := range sl.LogRecords {
					if r.Body.GetStringValue() == "client error" {
						found = true
						if r.TimeUnixNano != uint64(logAt.UnixNano()) || string(r.SpanId) != string(spanID.Bytes()) {
							t.Fatalf("LogAt record: time %d span %x", r.TimeUnixNano, r.SpanId)
						}
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("LogAt record not exported")
	}
}
