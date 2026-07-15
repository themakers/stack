package stack_stdlog

import (
	"context"
	"log/slog"

	"github.com/themakers/stack"
	"github.com/themakers/stack/stack_backend"
)

// NewSlogHandler creates a slog.Handler forwarding records into stack on top
// of ctx. The context is used as a fallback when the record does not carry its
// own (slog calls Handle with the caller's context).
func NewSlogHandler(ctx context.Context) slog.Handler {
	return slogHandler{ctx: ctx}
}

var _ slog.Handler = slogHandler{}

type slogHandler struct {
	ctx    context.Context
	attrs  []stack.A
	groups []string
}

func (h slogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	// Level filtering is the stack backends' job; pass everything through.
	return true
}

func (h slogHandler) Handle(ctx context.Context, record slog.Record) error {
	if ctx == nil {
		ctx = h.ctx
	}

	attrs := make([]stack.A, 0, len(h.attrs)+record.NumAttrs())
	attrs = append(attrs, h.attrs...)
	record.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, slogAttr(h.groups, a))
		return true
	})

	stack.Log(ctx, mapLevel(record.Level), record.Message, attrs...)
	return nil
}

func (h slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	merged := make([]stack.A, len(h.attrs), len(h.attrs)+len(attrs))
	copy(merged, h.attrs)
	for _, a := range attrs {
		merged = append(merged, slogAttr(h.groups, a))
	}
	h.attrs = merged
	return h
}

func (h slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	groups := make([]string, len(h.groups), len(h.groups)+1)
	copy(groups, h.groups)
	h.groups = append(groups, name)
	return h
}

func mapLevel(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return stack_backend.LevelError
	case level >= slog.LevelWarn:
		return stack_backend.LevelWarn
	case level >= slog.LevelInfo:
		return stack_backend.LevelInfo
	default:
		return stack_backend.LevelDebug
	}
}

// slogAttr converts a slog.Attr into a stack.A, expanding the name with groups
// (group.subgroup.key).
func slogAttr(groups []string, a slog.Attr) stack.A {
	name := a.Key
	for i := len(groups) - 1; i >= 0; i-- {
		name = groups[i] + "." + name
	}
	return stack.F(name, a.Value.Any())
}
