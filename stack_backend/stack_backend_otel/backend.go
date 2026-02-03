package stack_backend_otel

import (
	"context"

	common_model_v1 "go.opentelemetry.io/proto/otlp/common/v1"
	logs_model_v1 "go.opentelemetry.io/proto/otlp/logs/v1"
	metrics_model_v1 "go.opentelemetry.io/proto/otlp/metrics/v1"
	resource_model_v1 "go.opentelemetry.io/proto/otlp/resource/v1"
	trace_model_v1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/themakers/stack/stack_backend"
)

var _ stack_backend.Backend = Backend{}

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
			span.Events = append(span.Events, &trace_model_v1.Span_Event{
				Name:         l.Name,
				Attributes:   attrsToKeyValue(l.Attrs),
				TimeUnixNano: uint64(l.Time.UnixNano()),
			})
		}

		if e.State.Span.Error != nil {
			span.Status = &trace_model_v1.Status{
				Code:    trace_model_v1.Status_STATUS_CODE_ERROR,
				Message: e.State.Span.Error.Error(),
			}
		}

		if err := b.sink.ExportTraces(ctx, &trace_model_v1.TracesData{
			ResourceSpans: []*trace_model_v1.ResourceSpans{
				{
					Resource: &resource_model_v1.Resource{
						Attributes: attrsToKeyValue([]stack_backend.Attr{
							{Name: "service.name", Value: e.State.Options.ServiceName},
							{Name: "deployment.environment.name", Value: e.State.Options.Environment},
							{Name: "service.instance.id", Value: e.State.Options.Instance},
						}),
					},
					ScopeSpans: []*trace_model_v1.ScopeSpans{
						{
							// TODO
							//Scope: &common_model_v1.InstrumentationScope{
							//	Name:    "iscope-name-2",
							//	Version: "v0.0.1",
							//	Attributes: attrsToKeyValue([]stack_backend.Attr{
							//		{Name: "iscope-attr-1", Value: "adsfgsdfgsd"},
							//	}),
							//},
							Spans: []*trace_model_v1.Span{
								span,
							},
						},
					},
				},
			},
		}); err != nil {
			panic(err)
		}

	} else if e.Kind&stack_backend.KindLog != 0 {

		log := &logs_model_v1.LogRecord{
			Body: &common_model_v1.AnyValue{
				Value: &common_model_v1.AnyValue_StringValue{
					StringValue: e.LogEvent.Name,
				},
			},
			SeverityNumber: logs_model_v1.SeverityNumber_SEVERITY_NUMBER_INFO,
			SeverityText:   stack_backend.LevelInfo,
			Attributes:     attrsToKeyValue(e.LogEvent.OwnAttrs),
			TimeUnixNano:   uint64(e.LogEvent.Time.UnixNano()),
		}

		if !e.State.Span.ID.IsZero() {
			log.TraceId = e.State.Span.TraceID.Bytes()
			log.SpanId = e.State.Span.ID.Bytes()

			log.Attributes = append(log.Attributes, attrsToKeyValue(e.State.Span.Attrs)...)
		}

		if err := b.sink.ExportLogs(ctx, &logs_model_v1.LogsData{
			ResourceLogs: []*logs_model_v1.ResourceLogs{
				{
					Resource: &resource_model_v1.Resource{
						Attributes: attrsToKeyValue([]stack_backend.Attr{
							{Name: "service.name", Value: e.State.Options.ServiceName},
							{Name: "deployment.environment.name", Value: e.State.Options.Environment},
							{Name: "service.instance.id", Value: e.State.Options.Instance},
						}),
					},
					ScopeLogs: []*logs_model_v1.ScopeLogs{
						{
							LogRecords: []*logs_model_v1.LogRecord{
								log,
							},
						},
					},
				},
			},
		}); err != nil {
			panic(err)
		}

	}

}

func (b Backend) Shutdown(ctx context.Context) {
}
