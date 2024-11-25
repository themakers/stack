package stack

import (
	"context"

	"github.com/thearchitect/stack/stack_backend"
)

type Attr = stack_backend.Attr

func New(ctx context.Context, backend stack_backend.Backend) context.Context {
	return stack_backend.Put(ctx, stack_backend.Get(ctx).Clone(func(s *stack_backend.Span) {
		s.Backend = backend
	}))
}
