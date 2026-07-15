package stack_backend_otel

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	common_model_v1 "go.opentelemetry.io/proto/otlp/common/v1"
	logs_model_v1 "go.opentelemetry.io/proto/otlp/logs/v1"
	metrics_model_v1 "go.opentelemetry.io/proto/otlp/metrics/v1"
	resource_model_v1 "go.opentelemetry.io/proto/otlp/resource/v1"
	trace_model_v1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/themakers/stack/stack_backend"
)

var _ stack_backend.Backend = Backend{}

// exportErrLogInterval is the minimum interval between export error messages
// in stderr. Errors neither panic nor stay silent: an unreachable collector is
// visible in stderr without spamming it on every event.
const exportErrLogInterval = 10 * time.Second

var lastExportErrLog atomic.Int64

func reportExportError(kind string, err error) {
	if err == nil {
		return
	}
	now := time.Now().Unix()
	last := lastExportErrLog.Load()
	if now-last >= int64(exportErrLogInterval/time.Second) &&
		lastExportErrLog.CompareAndSwap(last, now) {
		fmt.Fprintf(os.Stderr, "stack: otel %s export error (further errors suppressed for %s): %v\n",
			kind, exportErrLogInterval, err)
	}
}

type OTLPSink interface {
	ExportLogs(ctx context.Context, ld *logs_model_v1.LogsData) error
	ExportMetrics(ctx context.Context, md *metrics_model_v1.MetricsData) error
	ExportTraces(ctx context.Context, td *trace_model_v1.TracesData) error
}

type Backend struct {
	sink OTLPSink
}

func New(sink OTLPSink) stack_backend.Backend {

	return Backend{
		sink: sink,
	}
}

func (b Backend) Handle(e stack_backend.Event) {
	ctx := context.Background()

	if e.Kind&stack_backend.KindSpanEnd != 0 {

		span := &trace_model_v1.Span{
			Name:              e.State.Span.Name,
			TraceId:           e.State.Span.TraceID.Bytes(),
			SpanId:            e.State.Span.ID.Bytes(),
			ParentSpanId:      e.State.Span.ParentSpanID.Bytes(),
			StartTimeUnixNano: uint64(e.State.Span.Time.UnixNano()),
			EndTimeUnixNano:   uint64(e.State.Span.EndTime.UnixNano()),
			Attributes:        attrsToKeyValue(e.State.Span.Attrs),
		}

		for _, l := range e.State.Span.OwnLogs {
			// Level/Error are stored as SpanLog fields (not in Attrs) — add
			// them as event attributes, otherwise the level and error of
			// nested logs would be lost in OTLP.
			attrs := l.Attrs
			if l.Level != "" {
				attrs = append(attrs[:len(attrs):len(attrs)], stack_backend.Attr{Name: "level", Value: l.Level})
			}
			if l.Error != nil {
				attrs = append(attrs[:len(attrs):len(attrs)], stack_backend.Attr{Name: "error", Value: l.Error.Error()})
			}
			span.Events = append(span.Events, &trace_model_v1.Span_Event{
				Name:         l.Name,
				Attributes:   attrsToKeyValue(attrs),
				TimeUnixNano: uint64(l.Time.UnixNano()),
			})
		}

		if e.State.Span.Error != nil {
			span.Status = &trace_model_v1.Status{
				Code:    trace_model_v1.Status_STATUS_CODE_ERROR,
				Message: e.State.Span.Error.Error(),
			}
		}

		// The logger must not panic when the collector is unreachable.
		reportExportError("traces", b.sink.ExportTraces(ctx, &trace_model_v1.TracesData{
			ResourceSpans: []*trace_model_v1.ResourceSpans{
				{
					Resource: b.resource(e),
					ScopeSpans: []*trace_model_v1.ScopeSpans{
						{
							Spans: []*trace_model_v1.Span{
								span,
							},
						},
					},
				},
			},
		}))

	} else if e.Kind&stack_backend.KindLog != 0 {

		log := &logs_model_v1.LogRecord{
			Body: &common_model_v1.AnyValue{
				Value: &common_model_v1.AnyValue_StringValue{
					StringValue: e.LogEvent.Name,
				},
			},
			SeverityNumber: severityNumber(e.LogEvent.Level),
			SeverityText:   e.LogEvent.Level,
			Attributes:     attrsToKeyValue(e.LogEvent.OwnAttrs),
			TimeUnixNano:   uint64(e.LogEvent.Time.UnixNano()),
		}

		// The log's error is exported as a separate attribute (it used to be lost entirely).
		if e.LogEvent.Error != nil {
			log.Attributes = append(log.Attributes, attrsToKeyValue([]stack_backend.Attr{
				{Name: "error", Value: e.LogEvent.Error.Error()},
			})...)
		}

		if !e.State.Span.ID.IsZero() {
			log.TraceId = e.State.Span.TraceID.Bytes()
			log.SpanId = e.State.Span.ID.Bytes()

			log.Attributes = append(log.Attributes, attrsToKeyValue(e.State.Span.Attrs)...)
		}

		reportExportError("logs", b.sink.ExportLogs(ctx, &logs_model_v1.LogsData{
			ResourceLogs: []*logs_model_v1.ResourceLogs{
				{
					Resource: b.resource(e),
					ScopeLogs: []*logs_model_v1.ScopeLogs{
						{
							LogRecords: []*logs_model_v1.LogRecord{
								log,
							},
						},
					},
				},
			},
		}))

	}

}

// resource builds the OTLP Resource from scope options. Includes the base
// service.* attributes and arbitrary ScopeAttrs (the latter used to not be
// exported at all).
func (b Backend) resource(e stack_backend.Event) *resource_model_v1.Resource {
	attrs := []stack_backend.Attr{
		{Name: "service.name", Value: e.State.Options.ServiceName},
		{Name: "deployment.environment", Value: e.State.Options.Environment},
		{Name: "service.instance.id", Value: e.State.Options.Instance},
	}
	attrs = append(attrs, e.State.Options.ScopeAttrs...)
	return &resource_model_v1.Resource{
		Attributes: attrsToKeyValue(attrs),
	}
}

// severityNumber maps the textual log level to the OTLP SeverityNumber
// (it used to be hardcoded to INFO).
func severityNumber(level string) logs_model_v1.SeverityNumber {
	switch level {
	case stack_backend.LevelDebug:
		return logs_model_v1.SeverityNumber_SEVERITY_NUMBER_DEBUG
	case stack_backend.LevelInfo:
		return logs_model_v1.SeverityNumber_SEVERITY_NUMBER_INFO
	case stack_backend.LevelWarn:
		return logs_model_v1.SeverityNumber_SEVERITY_NUMBER_WARN
	case stack_backend.LevelError:
		return logs_model_v1.SeverityNumber_SEVERITY_NUMBER_ERROR
	default:
		return logs_model_v1.SeverityNumber_SEVERITY_NUMBER_INFO
	}
}

func (b Backend) Shutdown(ctx context.Context) {
}
