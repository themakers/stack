package stack_backend

import "context"

type stackCtxKey struct{}

func Get(ctx context.Context) *Span {
	if s, ok := ctx.Value(stackCtxKey{}).(*Span); ok {
		return s
	} else {
		return nil
	}
}

func Clone(ctx context.Context, modFn func(s *Span)) *Span {
	return Get(ctx).Clone(modFn)
}

func Put(ctx context.Context, s *Span) context.Context {
	return context.WithValue(ctx, stackCtxKey{}, s)
}
