package stack_backend_otel

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	logs_model_v1 "go.opentelemetry.io/proto/otlp/logs/v1"
	resource_model_v1 "go.opentelemetry.io/proto/otlp/resource/v1"
	trace_model_v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/themakers/stack/stack_backend"
)

// Defaults follow the OTel SDK BatchSpanProcessor (queue 2048, batch 512);
// the flush interval is 1s — a single worker serves both spans and logs, so
// the interval leans towards the SDK's log processor rather than the 5s of
// the span processor.
const (
	defaultQueueSize     = 2048
	defaultBatchSize     = 512
	defaultFlushInterval = 1 * time.Second
)

var _ stack_backend.Backend = (*Batched)(nil)

// Batched is the batching OTel backend: Handle converts the event to an OTLP
// proto synchronously (the Event contract forbids retaining e.State) and
// enqueues it without blocking; a single background worker groups items and
// exports them in batches — by size (batchSize) or by timer (interval).
// When the queue is full, events are dropped and the fact is reported to
// stderr with rate limiting: telemetry must never block or crash the service.
type Batched struct {
	sink OTLPSink

	queue     chan batchItem
	batchSize int
	interval  time.Duration

	stopping atomic.Bool
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once

	dropped     atomic.Uint64
	lastDropLog atomic.Int64
}

type batchItem struct {
	resource *resource_model_v1.Resource
	span     *trace_model_v1.Span
	log      *logs_model_v1.LogRecord
}

// BatchOption configures NewBatched.
type BatchOption func(*Batched)

// WithBatchSize sets the maximum number of events exported in one call.
func WithBatchSize(n int) BatchOption {
	return func(b *Batched) {
		if n > 0 {
			b.batchSize = n
		}
	}
}

// WithFlushInterval sets the maximum time an event waits in the queue.
func WithFlushInterval(d time.Duration) BatchOption {
	return func(b *Batched) {
		if d > 0 {
			b.interval = d
		}
	}
}

// WithQueueSize sets the queue capacity; when full, new events are dropped.
func WithQueueSize(n int) BatchOption {
	return func(b *Batched) {
		if n > 0 {
			b.queue = make(chan batchItem, n)
		}
	}
}

// NewBatched creates the batching OTel backend and starts its worker. Panics
// on nil sink (startup configuration error — fail fast). Call
// stack_backend.Shutdown (or Backend.Shutdown directly) on process stop to
// drain the queue — otherwise the tail of telemetry is lost.
func NewBatched(sink OTLPSink, opts ...BatchOption) *Batched {
	if sink == nil {
		panic("stack_backend_otel: nil sink")
	}
	b := &Batched{
		sink:      sink,
		batchSize: defaultBatchSize,
		interval:  defaultFlushInterval,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
	for _, opt := range opts {
		opt(b)
	}
	if b.queue == nil {
		b.queue = make(chan batchItem, defaultQueueSize)
	}
	go b.worker()
	return b
}

func (b *Batched) Handle(e stack_backend.Event) {
	var it batchItem
	if e.Kind&stack_backend.KindSpanEnd != 0 {
		it = batchItem{resource: buildResource(e), span: convertSpan(e)}
	} else if e.Kind&stack_backend.KindLog != 0 {
		it = batchItem{resource: buildResource(e), log: convertLog(e)}
	} else {
		return
	}

	// After Shutdown started, the queue is being drained — new events are
	// dropped (same as events arriving into a full queue).
	if b.stopping.Load() {
		b.drop()
		return
	}

	select {
	case b.queue <- it:
	default:
		b.drop()
	}
}

func (b *Batched) drop() {
	n := b.dropped.Add(1)
	now := time.Now().Unix()
	last := b.lastDropLog.Load()
	if now-last >= int64(exportErrLogInterval/time.Second) &&
		b.lastDropLog.CompareAndSwap(last, now) {
		fmt.Fprintf(os.Stderr, "stack: otel batch queue full, %d events dropped so far (further reports suppressed for %s)\n",
			n, exportErrLogInterval)
	}
}

// Shutdown stops accepting events, drains the queue, flushes the remainder
// and waits for the worker until ctx expires.
func (b *Batched) Shutdown(ctx context.Context) {
	b.stopOnce.Do(func() {
		b.stopping.Store(true)
		close(b.stop)
	})
	select {
	case <-b.done:
	case <-ctx.Done():
	}
}

func (b *Batched) worker() {
	defer close(b.done)

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	batch := make([]batchItem, 0, b.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		b.export(batch)
		batch = batch[:0]
	}

	for {
		select {
		case it := <-b.queue:
			batch = append(batch, it)
			if len(batch) >= b.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-b.stop:
			// Drain whatever is already queued, flush and exit.
			for {
				select {
				case it := <-b.queue:
					batch = append(batch, it)
					if len(batch) >= b.batchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

// export groups the batch by resource (in practice constant for the process —
// a single bucket) and sends spans and logs in one export call each.
func (b *Batched) export(batch []batchItem) {
	var (
		resSpans []*trace_model_v1.ResourceSpans
		resLogs  []*logs_model_v1.ResourceLogs
	)

	for _, it := range batch {
		if it.span != nil {
			var bucket *trace_model_v1.ResourceSpans
			for _, rs := range resSpans {
				if proto.Equal(rs.Resource, it.resource) {
					bucket = rs
					break
				}
			}
			if bucket == nil {
				bucket = &trace_model_v1.ResourceSpans{
					Resource:   it.resource,
					ScopeSpans: []*trace_model_v1.ScopeSpans{{}},
				}
				resSpans = append(resSpans, bucket)
			}
			bucket.ScopeSpans[0].Spans = append(bucket.ScopeSpans[0].Spans, it.span)
		} else if it.log != nil {
			var bucket *logs_model_v1.ResourceLogs
			for _, rl := range resLogs {
				if proto.Equal(rl.Resource, it.resource) {
					bucket = rl
					break
				}
			}
			if bucket == nil {
				bucket = &logs_model_v1.ResourceLogs{
					Resource:  it.resource,
					ScopeLogs: []*logs_model_v1.ScopeLogs{{}},
				}
				resLogs = append(resLogs, bucket)
			}
			bucket.ScopeLogs[0].LogRecords = append(bucket.ScopeLogs[0].LogRecords, it.log)
		}
	}

	ctx := context.Background()
	if len(resSpans) > 0 {
		reportExportError("traces", b.sink.ExportTraces(ctx, &trace_model_v1.TracesData{ResourceSpans: resSpans}))
	}
	if len(resLogs) > 0 {
		reportExportError("logs", b.sink.ExportLogs(ctx, &logs_model_v1.LogsData{ResourceLogs: resLogs}))
	}
}
