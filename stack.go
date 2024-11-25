package stack

import (
	"context"

	"github.com/thearchitect/stack/span"
)

func Name(name string) option {
	return func(s *span.Span) {
		s.Name = name
	}
}

func A(name string, value any) Attr {
	return Attr{Name: name, Value: value}
}

func E(err error) Attr {
	return Attr{Name: "error", Value: err}
}

type option func(s *span.Span)

func With(ctx context.Context, opts ...option) context.Context {
	return span.Put(ctx, span.Clone(ctx, func(s *span.Span) {
		for _, o := range opts {
			o(s)
		}
	}))
}

////
////
////

// Trace is actually a 'Root Span'
// Used to pickup/continue Trace/Span from downstream
func Trace(ctx context.Context, traceID, parentSpanID string) (context.Context, context.CancelFunc) {
	ctx = span.Put(ctx, span.Clone(ctx, func(s *span.Span) {
		s.SetOrigin(traceID, parentSpanID)
		s.PushSpanID(span.GenerateID())
		s.FireSpan()
	}))

	return ctx, func() {
		span.Get(ctx).FireSpanEnd()
	}
}

func Span(ctx context.Context, ops ...option) (context.Context, context.CancelFunc) {
	ctx = span.Put(ctx, span.Clone(ctx, func(s *span.Span) {
		s.PushSpanID(span.GenerateID())
		s.Name, _, _ = operation(3)
		s.FireSpan()
	}))

	return ctx, func() {
		span.Get(ctx).FireSpanEnd()
	}
}

func Recover(ctx context.Context, rFn func(rec any)) {}

////
////
////

const (
	levelDebug = "debug"
	levelInfo  = "info"
	levelWarn  = "warn"
	levelError = "error"
)

func TLog(ctx context.Context, typed any) {
	// TODO
	//impl.Get(ctx).
}

func log(ctx context.Context, level, name string, err error, fields ...Attr) {
	span.Get(ctx).FireLog(name, level, err, fields)
}

func Log(ctx context.Context, level, name string, fields ...Attr) {
	log(ctx, level, name, nil, fields...)
}

func Debug(ctx context.Context, name string, fields ...Attr) {
	log(ctx, levelDebug, name, nil, fields...)
}

func Info(ctx context.Context, name string, fields ...Attr) {
	log(ctx, levelInfo, name, nil, fields...)
}

func Warn(ctx context.Context, name string, fields ...Attr) {
	log(ctx, levelWarn, name, nil, fields...)
}

func Error(ctx context.Context, name string, err error, fields ...Attr) {
	log(ctx, levelError, name, err, fields...)
}
