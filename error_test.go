package stack_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/themakers/stack"
	"github.com/themakers/stack/stack_backend"
)

type stackTraceProvider interface {
	error
	StackTrace() stack_backend.StackTrace
}

type tracedTestError struct{}

func (*tracedTestError) Error() string {
	return "boom"
}

func captureErrorAtOrigin(ctx context.Context, err error) error {
	return stack.Error(ctx, "origin failed", err)
}

func logErrorAtBoundary(ctx context.Context, err error) error {
	return stack.Error(ctx, "boundary failed", err)
}

func TestErrorPreservesFirstStackTrace(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)
	target := &tracedTestError{}

	originCtx, originDone := stack.Span(ctx)
	originErr := captureErrorAtOrigin(originCtx, target)
	originDone()
	if originErr == target {
		t.Fatal("Error returned the original error without a stack trace wrapper")
	}
	if !errors.Is(originErr, target) {
		t.Fatal("wrapped error does not preserve errors.Is")
	}
	if typed, ok := errors.AsType[*tracedTestError](originErr); !ok || typed != target {
		t.Fatal("wrapped error does not preserve errors.AsType")
	}
	traceProvider, ok := errors.AsType[stackTraceProvider](originErr)
	if !ok {
		t.Fatal("wrapped error does not expose its stack trace")
	}

	outerErr := fmt.Errorf("outer: %w", originErr)
	joinedErr := errors.Join(errors.New("unrelated"), outerErr)
	boundaryCtx, boundaryDone := stack.Span(ctx)
	if got := logErrorAtBoundary(boundaryCtx, joinedErr); got != joinedErr {
		t.Fatal("Error rewrapped an error whose tree already contains a stack trace")
	}
	boundaryDone()

	var traces []stack_backend.StackTrace
	for _, event := range cap.events {
		if event.Kind&stack_backend.KindError != 0 {
			traces = append(traces, event.LogEvent.StackTrace)
		}
	}
	if len(traces) != 2 {
		t.Fatalf("expected 2 error events, got %d", len(traces))
	}
	if !slices.Equal(traces[0], traces[1]) {
		t.Fatal("boundary error event did not reuse the origin stack trace")
	}
	if !slices.Equal(traceProvider.StackTrace(), traces[0]) {
		t.Fatal("error wrapper exposes a different stack trace than the origin event")
	}

	spanEnds := cap.spanEnds()
	if len(spanEnds) != 2 {
		t.Fatalf("expected 2 span-end events, got %d", len(spanEnds))
	}
	if !slices.Equal(spanEnds[1].State.Span.ErrorStackTrace, traces[0]) {
		t.Fatal("boundary span did not reuse the origin stack trace")
	}

	trace := traces[1].String()
	if !strings.Contains(trace, "captureErrorAtOrigin") {
		t.Fatalf("origin frame missing from preserved stack trace:\n%s", trace)
	}
	if strings.Contains(trace, "logErrorAtBoundary") {
		t.Fatalf("boundary frame replaced the origin stack trace:\n%s", trace)
	}
}

func TestErrorWithNilPreservesReturnContract(t *testing.T) {
	cap := &capture{}
	ctx := newCtx(cap)

	if err := stack.Error(ctx, "failed without an error", nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(cap.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(cap.events))
	}
	event := cap.events[0]
	if event.Kind&stack_backend.KindError == 0 {
		t.Fatalf("expected an error event, got kind %v", event.Kind)
	}
	if event.LogEvent.Error == nil {
		t.Fatal("nil error event does not contain a synthesized error")
	}
	if len(event.LogEvent.StackTrace) == 0 {
		t.Fatal("nil error event does not contain a stack trace")
	}
}
