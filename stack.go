package stack

import (
	"context"
	"fmt"
	"reflect"

	"github.com/thearchitect/stack/stack_backend"
)

//
// ▗▄▖ ▗▄▄▖▗▄▄▄▖▗▄▄▄▖ ▗▄▖ ▗▖  ▗▖ ▗▄▄▖
//▐▌ ▐▌▐▌ ▐▌ █    █  ▐▌ ▐▌▐▛▚▖▐▌▐▌
//▐▌ ▐▌▐▛▀▘  █    █  ▐▌ ▐▌▐▌ ▝▜▌ ▝▀▚▖
//▝▚▄▞▘▐▌    █  ▗▄█▄▖▝▚▄▞▘▐▌  ▐▌▗▄▄▞▘
//

func Name(name string) stack_backend.SpanOption {
	return stack_backend.SpanOptionFunc(func(s *stack_backend.Span) {
		s.Name = name
	})
}

func F(name string, value any) Attr {
	return Attr{Name: name, Value: value}
}

func E(err error) Attr { return F("error", err) }

// With TODO: Is this method helpful? Maybe just delete it?
func With(ctx context.Context, opts ...stack_backend.SpanOption) context.Context {
	return stack_backend.Put(ctx, stack_backend.Clone(ctx, func(s *stack_backend.Span) {
		for _, o := range opts {
			o.ApplyToSpan(s)
		}
	}))
}

//
//  ▗▄▄▖▗▄▄▖  ▗▄▖ ▗▖  ▗▖
// ▐▌   ▐▌ ▐▌▐▌ ▐▌▐▛▚▖▐▌
//  ▝▀▚▖▐▛▀▘ ▐▛▀▜▌▐▌ ▝▜▌
// ▗▄▄▞▘▐▌   ▐▌ ▐▌▐▌  ▐▌
//

type endFunc func()

// Trace is actually a 'Root Span'
// Used to pickup/continue Trace/Span from downstream
func Trace(ctx context.Context, traceID, parentSpanID string, opts ...stack_backend.SpanOption) (context.Context, endFunc) {
	ctx = stack_backend.Put(ctx, stack_backend.Clone(ctx, func(s *stack_backend.Span) {
		s.SetOrigin(traceID, parentSpanID)
		s.PushSpanID(stack_backend.GenerateID())
		for _, o := range opts {
			o.ApplyToSpan(s)
		}
		s.FireSpan()
	}))

	return ctx, func() {
		stack_backend.Get(ctx).FireSpanEnd()
	}
}

func Span(ctx context.Context, opts ...stack_backend.SpanOption) (context.Context, endFunc) {
	ctx = stack_backend.Put(ctx, stack_backend.Clone(ctx, func(s *stack_backend.Span) {
		s.PushSpanID(stack_backend.GenerateID())
		s.Name, _, _ = operation(3)
		s.Name = fmt.Sprint(s.Name, "()") // ???
		for _, o := range opts {
			o.ApplyToSpan(s)
		}
		s.FireSpan()
	}))

	return ctx, func() {
		stack_backend.Get(ctx).FireSpanEnd()
	}
}

//
// ▗▖    ▗▄▖  ▗▄▄▖ ▗▄▄▖▗▄▄▄▖▗▖  ▗▖ ▗▄▄▖
// ▐▌   ▐▌ ▐▌▐▌   ▐▌     █  ▐▛▚▖▐▌▐▌
// ▐▌   ▐▌ ▐▌▐▌▝▜▌▐▌▝▜▌  █  ▐▌ ▝▜▌▐▌▝▜▌
// ▐▙▄▄▖▝▚▄▞▘▝▚▄▞▘▝▚▄▞▘▗▄█▄▖▐▌  ▐▌▝▚▄▞▘
//

func log(ctx context.Context, level, name string, err error, fields ...Attr) {
	stack_backend.Get(ctx).FireLog(name, level, err, fields)
}

func Log(ctx context.Context, level, name string, fields ...Attr) {
	log(ctx, level, name, nil, fields...)
}

func Debug(ctx context.Context, name string, fields ...Attr) {
	log(ctx, stack_backend.LevelDebug, name, nil, fields...)
}

func Info(ctx context.Context, name string, fields ...Attr) {
	log(ctx, stack_backend.LevelInfo, name, nil, fields...)
}

func Warn(ctx context.Context, name string, fields ...Attr) {
	log(ctx, stack_backend.LevelWarn, name, nil, fields...)
}

func Error(ctx context.Context, name string, err error, fields ...Attr) {
	log(ctx, stack_backend.LevelError, name, err, fields...)
}

func TLog(ctx context.Context, typed any) {
	var (
		val = reflect.ValueOf(typed)
		typ = val.Type()
	)

	for typ.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = val.Type()
	}

	if typ.Kind() != reflect.Struct {
		// FIXME: Log instead of panic
		panic("input must be a struct or pointer to a struct")
	}

	fullName := fmt.Sprintf("%s.%s", typ.PkgPath(), typ.Name())

	var attrs []Attr
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		if !fieldValue.CanInterface() {
			continue
		}

		attrs = append(attrs, Attr{
			Name:  field.Name,
			Value: fieldValue.Interface(),
		})
	}

	log(ctx, stack_backend.LevelDebug, fullName, nil, attrs...)
}

//
//▗▄▄▄▖▗▄▄▖ ▗▄▄▖  ▗▄▖ ▗▄▄▖  ▗▄▄▖
//▐▌   ▐▌ ▐▌▐▌ ▐▌▐▌ ▐▌▐▌ ▐▌▐▌
//▐▛▀▀▘▐▛▀▚▖▐▛▀▚▖▐▌ ▐▌▐▛▀▚▖ ▝▀▚▖
//▐▙▄▄▖▐▌ ▐▌▐▌ ▐▌▝▚▄▞▘▐▌ ▐▌▗▄▄▞▘
//

func Recover(ctx context.Context, rFn func(rec any)) {

}
