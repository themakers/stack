package stack

import (
	"context"

	"github.com/thearchitect/stack/span"
	"github.com/thearchitect/stack/stack_backend"
)

type Attr = stack_backend.Attr

func New(ctx context.Context, backend stack_backend.Backend) context.Context {
	return span.Put(ctx, span.Get(ctx).Clone(func(s *span.Span) {
		s.Backend = backend
	}))
}
