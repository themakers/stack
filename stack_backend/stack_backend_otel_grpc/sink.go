package stack_backend_otel_grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	logs_v1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metrics_v1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	trace_v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	logs_model_v1 "go.opentelemetry.io/proto/otlp/logs/v1"
	metrics_model_v1 "go.opentelemetry.io/proto/otlp/metrics/v1"
	trace_model_v1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/themakers/stack/stack_backend/stack_backend_otel"
)

var _ stack_backend_otel.OTLPSink = (*Sink)(nil)

type Sink struct {
	cc      *grpc.ClientConn
	logs    logs_v1.LogsServiceClient
	traces  trace_v1.TraceServiceClient
	metrics metrics_v1.MetricsServiceClient
}

func New(target string) stack_backend_otel.OTLPSink {
	cc, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	return &Sink{
		cc:      cc,
		logs:    logs_v1.NewLogsServiceClient(cc),
		traces:  trace_v1.NewTraceServiceClient(cc),
		metrics: metrics_v1.NewMetricsServiceClient(cc),
	}
}

func (s *Sink) ExportLogs(ctx context.Context, ld *logs_model_v1.LogsData) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := s.logs.Export(ctx, &logs_v1.ExportLogsServiceRequest{
		ResourceLogs: ld.ResourceLogs,
	}); err != nil {
		panic(err)
	} else {
		return nil
	}
}

func (s *Sink) ExportTraces(ctx context.Context, td *trace_model_v1.TracesData) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := s.traces.Export(ctx, &trace_v1.ExportTraceServiceRequest{
		ResourceSpans: td.ResourceSpans,
	}); err != nil {
		panic(err)
	} else {
		return nil
	}
}

func (s *Sink) ExportMetrics(ctx context.Context, md *metrics_model_v1.MetricsData) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := s.metrics.Export(ctx, &metrics_v1.ExportMetricsServiceRequest{
		ResourceMetrics: md.ResourceMetrics,
	}); err != nil {
		panic(err)
	} else {
		return nil
	}
}
