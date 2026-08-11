package stack_backend_otel_test

import (
	"context"
	"errors"
	"testing"

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
