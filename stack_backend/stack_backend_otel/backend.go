package stack_backend_otel

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
}

func New(target string) (stack_backend.Backend, func()) {
	cc, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	return Backend{
			traces: trace_v1.NewTraceServiceClient(cc),
			logs:   logs_v1.NewLogsServiceClient(cc),
		}, func() {
			if err := cc.Close(); err != nil {
				panic(err)
			}
		}
}

func (b Backend) Handle(e stack_backend.Event) {
	if e.Kind&stack_backend.KindSpanEnd == 0 {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if res, err := b.traces.Export(ctx, &trace_v1.ExportTraceServiceRequest{
		ResourceSpans: []*trace_model_v1.ResourceSpans{
			{
				Resource: &resource_model_v1.Resource{},
				ScopeSpans: []*trace_model_v1.ScopeSpans{
					{
						Scope: &common_model_v1.InstrumentationScope{},
						Spans: []*trace_model_v1.Span{
							{
								Name:              e.Name,
								TraceId:           e.TraceID[:],
								SpanId:            e.ID[:],
								ParentSpanId:      e.ParentID[:],
								StartTimeUnixNano: uint64(e.Time.UnixNano()),
								EndTimeUnixNano:   uint64(e.EndTime.UnixNano()),
								Attributes:        attrsToKeyValue(e.Attrs),
								Events: []*trace_model_v1.Span_Event{
									{
										Attributes: []*common_model_v1.KeyValue{},
									},
								},
							},
						},
					},
				},
			},
		},
	}); err != nil {
		panic(err)
	} else {
		println(res.PartialSuccess)
	}

	if false {
		if res, err := b.logs.Export(ctx, &logs_v1.ExportLogsServiceRequest{
			ResourceLogs: []*logs_model_v1.ResourceLogs{
				{
					Resource: &resource_model_v1.Resource{},
					ScopeLogs: []*logs_model_v1.ScopeLogs{
						{
							Scope: &common_model_v1.InstrumentationScope{},
							LogRecords: []*logs_model_v1.LogRecord{
								{
									Attributes: []*common_model_v1.KeyValue{},
								},
							},
						},
					},
				},
			},
		}); err != nil {
			panic(err)
		} else {
			println(res.PartialSuccess)
		}
	}
}
