package stack_stdlog

import (
	"context"
	"log/slog"
)

func NewSlogHandler(ctx context.Context) slog.Handler {
	return slogHandler{}
}

var _ slog.Handler = slogHandler{}

type slogHandler struct{}

func (h slogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	//TODO implement me
	panic("implement me")
}

func (h slogHandler) Handle(ctx context.Context, record slog.Record) error {
	//TODO implement me
	panic("implement me")
}

func (h slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	//TODO implement me
	panic("implement me")
}

func (h slogHandler) WithGroup(name string) slog.Handler {
	//TODO implement me
	panic("implement me")
}
