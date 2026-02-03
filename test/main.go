package main

import (
	"context"
	"errors"
	"time"

	"github.com/themakers/stack"
	"github.com/themakers/stack/stack_backend"
	"github.com/themakers/stack/stack_backend/stack_backend_otel_grpc_legacy"
	"github.com/themakers/stack/stack_backend/stack_backend_text"
	"github.com/themakers/stack/stack_stdlog"

	"github.com/themakers/stack/test/log_events"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = stack.With().Backend(stack_backend.TeeBackend(
		stack_backend_text.New(),
		//stack_backend_json.New(),
		stack_backend_otel_grpc_legacy.New("localhost:32751"),
	)).ServiceName("stack-demo").ScopeAttrs(stack.Attr("build", "0xdeadbeef")).Apply(ctx)

	defer stack_backend.Shutdown(ctx)

	stack_stdlog.Hijack(ctx)

	ctx, end := stack.Span(ctx, stack.Attr("buildnum", 100500))
	defer end()

	stack.Log(ctx, "ajajajajajaja", "test", stack.Attr("test", map[string]any{"hello": "kitty", "bananas": 10}))

	time.Sleep(3 * time.Millisecond)

	stack.TLog(ctx, log_events.TestRecord{
		Name:         "j doe",
		RegisteredAt: time.Now(),
	})

	(func() {
		ctx, end := stack.Span(ctx, stack.Name("spaaaaana"))
		defer end()

		time.Sleep(3 * time.Millisecond)

		stack.Info(ctx, "hello kitty", stack.Attr("user_name", "kenji kawai"))

		SpaaaaaaaaaanFunc(ctx)
	})()
}

func SpaaaaaaaaaanFunc(ctx context.Context) {
	ctx, end := stack.Span(ctx)
	defer end()

	stack.Error(ctx, "woooooork", errors.New("test-error"))

	time.Sleep(10 * time.Millisecond)
}
