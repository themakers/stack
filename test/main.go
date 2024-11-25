package main

import (
	"context"
	"errors"
	"time"

	"github.com/thearchitect/stack"
	"github.com/thearchitect/stack/stack_backend/stack_backend_text"
	"github.com/thearchitect/stack/stack_stdlog"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = stack.New(ctx, stack_backend_text.New())
	stack_stdlog.Hijack(ctx)

	ctx, cancel = stack.Span(ctx, stack.AA("buildnum", 100500))
	defer cancel()

	(func() {
		ctx, cancel := stack.Span(ctx, stack.Name("spaaaaana"))
		defer cancel()

		stack.Info(ctx, "hello kitty", stack.A("user_name", "kenji kawai"))

		SpaaaaaaaaaanFunc(ctx)
	})()
}

func SpaaaaaaaaaanFunc(ctx context.Context) {
	ctx, cancel := stack.Span(ctx)
	defer cancel()

	stack.Error(ctx, "woooooork", errors.New("test-error"))

	time.Sleep(10 * time.Millisecond)
}
