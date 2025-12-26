package stack_backend_otel_grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	logs_v1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	trace_v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	common_model_v1 "go.opentelemetry.io/proto/otlp/common/v1"
	logs_model_v1 "go.opentelemetry.io/proto/otlp/logs/v1"
	resource_model_v1 "go.opentelemetry.io/proto/otlp/resource/v1"
	trace_model_v1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/themakers/stack/stack_backend"
)

var _ stack_backend.Backend = Backend{}

type Backend struct {
	traces trace_v1.TraceServiceClient
	logs   logs_v1.LogsServiceClient
	cc     *grpc.ClientConn
}

func New(target string) stack_backend.Backend {
	cc, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	return Backend{
		cc:     cc,
		traces: trace_v1.NewTraceServiceClient(cc),
		logs:   logs_v1.NewLogsServiceClient(cc),
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

		if _, err := b.traces.Export(ctx, &trace_v1.ExportTraceServiceRequest{
			ResourceSpans: []*trace_model_v1.ResourceSpans{
				{
					Resource: &resource_model_v1.Resource{
						Attributes: attrsToKeyValue([]stack_backend.Attr{
							{Name: "service.name", Value: e.State.Options.ServiceName},
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
		} else {
			//println(res.PartialSuccess)
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

		if _, err := b.logs.Export(ctx, &logs_v1.ExportLogsServiceRequest{
			ResourceLogs: []*logs_model_v1.ResourceLogs{
				{
					Resource: &resource_model_v1.Resource{
						Attributes: attrsToKeyValue([]stack_backend.Attr{
							{Name: "service.name", Value: e.State.Options.ServiceName},
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
		} else {
			//println(res.PartialSuccess)
		}

	}

}

func (b Backend) Shutdown(ctx context.Context) {
	b.cc.Close()
}
