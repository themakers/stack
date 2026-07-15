package stack_backend_otel_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	logs_model_v1 "go.opentelemetry.io/proto/otlp/logs/v1"
	metrics_model_v1 "go.opentelemetry.io/proto/otlp/metrics/v1"
	trace_model_v1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/themakers/stack"
	"github.com/themakers/stack/stack_backend/stack_backend_otel"
)

type fakeSink struct {
	traces []*trace_model_v1.TracesData
	logs   []*logs_model_v1.LogsData
}

func (s *fakeSink) ExportLogs(_ context.Context, ld *logs_model_v1.LogsData) error {
	s.logs = append(s.logs, ld)
	return nil
}

func (s *fakeSink) ExportMetrics(_ context.Context, _ *metrics_model_v1.MetricsData) error {
	return nil
}

func (s *fakeSink) ExportTraces(_ context.Context, td *trace_model_v1.TracesData) error {
	s.traces = append(s.traces, td)
	return nil
}

// A failed span exports an OTel semconv "exception" event carrying the
// resolved stack trace; an error log record carries exception.stacktrace.
func TestExceptionStackTraceExported(t *testing.T) {
	sink := &fakeSink{}
	ctx := stack.With().Backend(stack_backend_otel.New(sink)).Apply(context.Background())

	c, done := stack.Span(ctx)
	stack.Error(c, "explosion", errors.New("bang"))
	done()

	if len(sink.logs) == 0 || len(sink.traces) == 0 {
		t.Fatalf("expected exports: logs=%d traces=%d", len(sink.logs), len(sink.traces))
	}

	// Log record: exception.stacktrace attribute with a resolved frame.
	logAttrs := map[string]string{}
	for _, rl := range sink.logs[0].ResourceLogs {
		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				for _, kv := range lr.Attributes {
					logAttrs[kv.Key] = kv.Value.GetStringValue()
				}
			}
		}
	}
	if st := logAttrs["exception.stacktrace"]; !strings.Contains(st, "TestExceptionStackTraceExported") {
		t.Errorf("log exception.stacktrace missing test frame:\n%s", st)
	}
	if logAttrs["error"] != "bang" {
		t.Errorf("log error attr = %q", logAttrs["error"])
	}

	// Span: status ERROR + "exception" event with message and stacktrace.
	var span *trace_model_v1.Span
	for _, rs := range sink.traces[0].ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				span = sp
			}
		}
	}
	if span == nil {
		t.Fatal("no span exported")
	}
	if span.Status.GetCode() != trace_model_v1.Status_STATUS_CODE_ERROR {
		t.Errorf("span status = %v", span.Status)
	}
	var exc *trace_model_v1.Span_Event
	for _, ev := range span.Events {
		if ev.Name == "exception" {
			exc = ev
		}
	}
	if exc == nil {
		t.Fatal("no exception span event")
	}
	excAttrs := map[string]string{}
	for _, kv := range exc.Attributes {
		excAttrs[kv.Key] = kv.Value.GetStringValue()
	}
	if excAttrs["exception.message"] != "bang" {
		t.Errorf("exception.message = %q", excAttrs["exception.message"])
	}
	if st := excAttrs["exception.stacktrace"]; !strings.Contains(st, "TestExceptionStackTraceExported") {
		t.Errorf("exception.stacktrace missing test frame:\n%s", st)
	}
}

// Typed Value kinds map to native OTLP value types.
func TestTypedOTLPValues(t *testing.T) {
	sink := &fakeSink{}
	ctx := stack.With().Backend(stack_backend_otel.New(sink)).Apply(context.Background())

	stack.Info(ctx, "typed",
		stack.F("n", 42),
		stack.F("f", 2.5),
		stack.F("ok", true),
		stack.F("s", "str"),
	)

	if len(sink.logs) == 0 {
		t.Fatal("no logs exported")
	}
	found := map[string]bool{}
	for _, rl := range sink.logs[0].ResourceLogs {
		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				for _, kv := range lr.Attributes {
					switch kv.Key {
					case "n":
						found["n"] = kv.Value.GetIntValue() == 42
					case "f":
						found["f"] = kv.Value.GetDoubleValue() == 2.5
					case "ok":
						found["ok"] = kv.Value.GetBoolValue()
					case "s":
						found["s"] = kv.Value.GetStringValue() == "str"
					}
				}
			}
		}
	}
	for _, k := range []string{"n", "f", "ok", "s"} {
		if !found[k] {
			t.Errorf("attr %q: wrong OTLP value type or value", k)
		}
	}
}
