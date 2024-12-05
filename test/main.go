package main

import (
	"context"
	"errors"
	"github.com/themakers/stack/stack_backend/stack_backend_json"
	"github.com/themakers/stack/stack_backend/stack_backend_otel"
	"time"

	"github.com/themakers/stack"
	"github.com/themakers/stack/stack_backend"
	"github.com/themakers/stack/stack_backend/stack_backend_text"
	"github.com/themakers/stack/stack_stdlog"

	"github.com/themakers/stack/test/log_events"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backendOTEL, closeBackendOTEL := stack_backend_otel.New("localhost:32751")
	defer closeBackendOTEL()

	ctx = stack.New(ctx,
		stack_backend.TeeBackend(
			stack_backend_text.New(),
			stack_backend_json.New(),
			backendOTEL,
		),
	)
	stack_stdlog.Hijack(ctx)

	ctx, end := stack.Span(ctx, stack.A("buildnum", 100500))
	defer end()

	stack.Log(ctx, "ajajajajajaja", "test", stack.A("test", map[string]any{"hello": "kitty", "bananas": 10}))

	stack.TLog(ctx, log_events.TestRecord{
		Name:         "j doe",
		RegisteredAt: time.Now(),
	})

	(func() {
		ctx, cancel := stack.Span(ctx, stack.AddName("spaaaaana"))
		defer cancel()

		stack.Info(ctx, "hello kitty", stack.A("user_name", "kenji kawai"))

		SpaaaaaaaaaanFunc(ctx)
	})()
}

func SpaaaaaaaaaanFunc(ctx context.Context) {
	ctx, end := stack.Span(ctx)
	defer end()

	stack.Error(ctx, "woooooork", errors.New("test-error"))

	//err := _drafts.NewError(ctx, err, stack.A(), stack.A(), stack.A())
	//if IsMND(err) {
	//	MNDE{err}
	//} else {
	//	ISE{err}
	//}

	time.Sleep(10 * time.Millisecond)
}

type PermissionDeniedError struct {
	cause error
}

type MNDE struct {
	cause error
}

type ISE struct {
	cause error
}
