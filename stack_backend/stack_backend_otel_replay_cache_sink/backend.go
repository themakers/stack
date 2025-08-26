package stack_backend_otel_replay_cache_sink

import (
	"context"

	"github.com/themakers/replay/lib/cache"
	logs_model_v1 "go.opentelemetry.io/proto/otlp/logs/v1"
	metrics_model_v1 "go.opentelemetry.io/proto/otlp/metrics/v1"
	trace_model_v1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/themakers/stack/stack_backend/stack_backend_otel"
)

var _ stack_backend_otel.OTLPSink = Backend{}

type Backend struct {
	sink cache.CacheIngest
}

func New(ingest cache.CacheIngest) stack_backend_otel.OTLPSink {
	return Backend{
		sink: ingest,
	}
}

func (b Backend) ExportLogs(ctx context.Context, ld *logs_model_v1.LogsData) error {
	b.sink.PutLogs(ld)
	return nil
}

func (b Backend) ExportMetrics(ctx context.Context, md *metrics_model_v1.MetricsData) error {
	b.sink.PutMetrics(md)
	return nil
}

func (b Backend) ExportTraces(ctx context.Context, td *trace_model_v1.TracesData) error {
	b.sink.PutTraces(td)
	return nil
}
