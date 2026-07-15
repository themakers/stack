package stack_backend_otel_test

import (
	"context"
	"sync"
	"testing"
	"time"

	logs_model_v1 "go.opentelemetry.io/proto/otlp/logs/v1"
	metrics_model_v1 "go.opentelemetry.io/proto/otlp/metrics/v1"
	trace_model_v1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/themakers/stack"
	"github.com/themakers/stack/stack_backend/stack_backend_otel"
)

// syncSink is a fake sink safe for concurrent use by the batch worker.
type syncSink struct {
	mu          sync.Mutex
	traceCalls  int
	logCalls    int
	spans       int
	logs        int
	block       chan struct{} // when set, ExportTraces blocks until closed
	exportedSig chan struct{} // signaled on every export call
}

func newSyncSink() *syncSink {
	return &syncSink{exportedSig: make(chan struct{}, 64)}
}

func (s *syncSink) signal() {
	select {
	case s.exportedSig <- struct{}{}:
	default:
	}
}

func (s *syncSink) ExportLogs(_ context.Context, ld *logs_model_v1.LogsData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logCalls++
	for _, rl := range ld.ResourceLogs {
		for _, sl := range rl.ScopeLogs {
			s.logs += len(sl.LogRecords)
		}
	}
	s.signal()
	return nil
}

func (s *syncSink) ExportMetrics(_ context.Context, _ *metrics_model_v1.MetricsData) error {
	return nil
}

func (s *syncSink) ExportTraces(_ context.Context, td *trace_model_v1.TracesData) error {
	if s.block != nil {
		<-s.block
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traceCalls++
	for _, rs := range td.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			s.spans += len(ss.Spans)
		}
	}
	s.signal()
	return nil
}

func (s *syncSink) snapshot() (traceCalls, logCalls, spans, logs int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.traceCalls, s.logCalls, s.spans, s.logs
}

// A batch flushes when it reaches batchSize, without waiting for the timer.
func TestBatchFlushOnSize(t *testing.T) {
	sink := newSyncSink()
	backend := stack_backend_otel.NewBatched(sink,
		stack_backend_otel.WithBatchSize(3),
		stack_backend_otel.WithFlushInterval(time.Hour),
	)
	defer backend.Shutdown(context.Background())

	ctx := stack.With().Backend(backend).Apply(context.Background())
	for range 3 {
		_, done := stack.Span(ctx)
		done()
	}

	select {
	case <-sink.exportedSig:
	case <-time.After(5 * time.Second):
		t.Fatal("batch was not flushed on size")
	}

	traceCalls, _, spans, _ := sink.snapshot()
	if traceCalls != 1 || spans != 3 {
		t.Fatalf("traceCalls=%d spans=%d, want 1/3 (one batched export)", traceCalls, spans)
	}
}

// A pending batch flushes on the timer tick.
func TestBatchFlushOnInterval(t *testing.T) {
	sink := newSyncSink()
	backend := stack_backend_otel.NewBatched(sink,
		stack_backend_otel.WithFlushInterval(50*time.Millisecond),
	)
	defer backend.Shutdown(context.Background())

	ctx := stack.With().Backend(backend).Apply(context.Background())
	stack.Info(ctx, "ping")

	select {
	case <-sink.exportedSig:
	case <-time.After(5 * time.Second):
		t.Fatal("batch was not flushed on interval")
	}

	if _, logCalls, _, logs := sink.snapshot(); logCalls != 1 || logs != 1 {
		t.Fatalf("logCalls=%d logs=%d, want 1/1", logCalls, logs)
	}
}

// Shutdown drains everything still in the queue.
func TestBatchShutdownDrains(t *testing.T) {
	sink := newSyncSink()
	backend := stack_backend_otel.NewBatched(sink,
		stack_backend_otel.WithFlushInterval(time.Hour),
		stack_backend_otel.WithBatchSize(1000),
	)

	ctx := stack.With().Backend(backend).Apply(context.Background())
	const n = 25
	for range n {
		stack.Info(ctx, "queued")
	}

	backend.Shutdown(context.Background())

	if _, _, _, logs := sink.snapshot(); logs != n {
		t.Fatalf("drained logs = %d, want %d", logs, n)
	}
}

// A full queue drops events instead of blocking Handle.
func TestBatchDropWhenFull(t *testing.T) {
	sink := newSyncSink()
	sink.block = make(chan struct{})

	backend := stack_backend_otel.NewBatched(sink,
		stack_backend_otel.WithBatchSize(1),
		stack_backend_otel.WithQueueSize(1),
		stack_backend_otel.WithFlushInterval(time.Hour),
	)

	ctx := stack.With().Backend(backend).Apply(context.Background())

	// The first span goes to the worker and blocks in ExportTraces; the queue
	// (cap 1) fills up; the rest must be dropped without blocking.
	const n = 50
	start := time.Now()
	for range n {
		_, done := stack.Span(ctx)
		done()
	}
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("Handle blocked on a full queue: %v for %d events", elapsed, n)
	}

	close(sink.block)
	backend.Shutdown(context.Background())

	_, _, spans, _ := sink.snapshot()
	if spans == 0 || spans >= n {
		t.Fatalf("spans exported = %d, want >0 and <%d (some dropped)", spans, n)
	}
}

// Concurrent Handle calls and Shutdown — race detector coverage.
func TestBatchConcurrency(t *testing.T) {
	sink := newSyncSink()
	// The queue holds the full volume: with drop-on-full semantics the
	// "nothing lost" assertion is only valid when nothing overflows.
	backend := stack_backend_otel.NewBatched(sink,
		stack_backend_otel.WithFlushInterval(time.Millisecond),
		stack_backend_otel.WithQueueSize(8*200*2),
	)

	ctx := stack.With().Backend(backend).Apply(context.Background())

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 200 {
				c, done := stack.Span(ctx)
				stack.Info(c, "concurrent")
				done()
			}
		})
	}
	wg.Wait()
	backend.Shutdown(context.Background())

	if _, _, spans, logs := sink.snapshot(); spans != 8*200 || logs != 8*200 {
		t.Fatalf("spans=%d logs=%d, want 1600/1600 (nothing lost)", spans, logs)
	}
}

type discardSink struct{}

func (discardSink) ExportLogs(context.Context, *logs_model_v1.LogsData) error { return nil }
func (discardSink) ExportMetrics(context.Context, *metrics_model_v1.MetricsData) error {
	return nil
}
func (discardSink) ExportTraces(context.Context, *trace_model_v1.TracesData) error { return nil }

// BenchmarkBatchedHandle — the hot-path cost of the batched backend: proto
// conversion + non-blocking enqueue, no network on the caller's goroutine.
func BenchmarkBatchedHandle(b *testing.B) {
	backend := stack_backend_otel.NewBatched(discardSink{})
	defer backend.Shutdown(context.Background())

	ctx := stack.With().Backend(backend).Apply(context.Background())
	b.ReportAllocs()
	
	for b.Loop() {
		_, done := stack.Span(ctx)
		done()
	}
}
