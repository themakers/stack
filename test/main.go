package main

import (
	"context"
	"errors"
	"github.com/thearchitect/stack/stack_stdlog"

	"github.com/thearchitect/stack"
	"github.com/thearchitect/stack/stack_backend/stack_backend_json"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = stack.New(ctx, stack_backend_json.New())
	stack_stdlog.Hijack(ctx)

	ctx, cancel = stack.Span(ctx)
	defer cancel()

	(func() {
		ctx, cancel := stack.Span(ctx)
		defer cancel()

		stack.Info(ctx, "hello kitty", stack.A("user_name", "kenji kawai"))

		SpaaaaaaaaaanFunc(ctx)
	})()
}

func SpaaaaaaaaaanFunc(ctx context.Context) {
	ctx, cancel := stack.Span(ctx)
	defer cancel()

	stack.Error(ctx, "woooooork", errors.New("test-error"))
}
