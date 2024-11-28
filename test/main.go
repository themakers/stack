package main

import (
	"context"
	"errors"
	"time"

	"github.com/thearchitect/stack"
	"github.com/thearchitect/stack/stack_backend/stack_backend_text"
	"github.com/thearchitect/stack/stack_stdlog"

	"github.com/thearchitect/stack/test/log_events"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = stack.New(ctx, stack_backend_text.New())
	stack_stdlog.Hijack(ctx)

	ctx, end := stack.Span(ctx, stack.A("buildnum", 100500))
	defer end()

	stack.Log(ctx, "ajajajajajaja", "test", stack.A("test", map[string]any{"hello": "kitty", "bananas": 10}))

	stack.TLog(ctx, log_events.TestRecord{
		Name:         "j doe",
		RegisteredAt: time.Now(),
	})

	(func() {
		ctx, cancel := stack.Span(ctx, stack.Name("spaaaaana"))
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
