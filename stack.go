package stack

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/themakers/stack/stack_backend"
)

type A = stack_backend.Attr

func Attr(name string, value any) A {
	return A{Name: name, Value: value}
}

// COMMON
//  ▗▄▖ ▗▄▄▖▗▄▄▄▖▗▄▄▄▖ ▗▄▖ ▗▖  ▗▖ ▗▄▄▖
// ▐▌ ▐▌▐▌ ▐▌ █    █  ▐▌ ▐▌▐▛▚▖▐▌▐▌
// ▐▌ ▐▌▐▛▀▘  █    █  ▐▌ ▐▌▐▌ ▝▜▌ ▝▀▚▖
// ▝▚▄▞▘▐▌    █  ▗▄█▄▖▝▚▄▞▘▐▌  ▐▌▗▄▄▞▘
//

func Name(name string) stack_backend.Option {
	return stack_backend.OptionFunc(func(s *stack_backend.Stack) {
		s.Span.Name = name
	})
}

func With() stack_backend.Options {
	return stack_backend.Options{}
}

//
//  ▗▄▄▖▗▄▄▖  ▗▄▖ ▗▖  ▗▖
// ▐▌   ▐▌ ▐▌▐▌ ▐▌▐▛▚▖▐▌
//  ▝▀▚▖▐▛▀▘ ▐▛▀▜▌▐▌ ▝▜▌
// ▗▄▄▞▘▐▌   ▐▌ ▐▌▐▌  ▐▌
//

type endFunc func()

func Span(ctx context.Context, opts ...stack_backend.Option) (context.Context, endFunc) {
	var s = stack_backend.Get(ctx).Clone()

	s.Span.Time = time.Now()
	s.Span.OwnLogs = []stack_backend.SpanLog{}

	if s.Span.TraceID.IsZero() {
		s.Span.TraceID = stack_backend.NewTraceID()
	}

	s.Span.ParentSpanID = s.Span.ID
	s.Span.ID = stack_backend.NewID()

	s.Span.Name, _, _ = stack_backend.Operation(0)
	s.Span.Name = fmt.Sprint(s.Span.Name, "()") // ???

	//> Apply options from arguments
	stack_backend.Options(opts).ApplyToStack(s)

	ctx = stack_backend.Put(ctx, s)

	s.Backend.Handle(stack_backend.Event{
		Kind:  stack_backend.KindSpan,
		State: s,
	})

	return ctx, func() {
		s.Span.EndTime = time.Now()
		s.Backend.Handle(stack_backend.Event{
			Kind:  stack_backend.KindSpanEnd,
			State: s,
		})
	}
}

//
// ▗▖    ▗▄▖  ▗▄▄▖ ▗▄▄▖▗▄▄▄▖▗▖  ▗▖ ▗▄▄▖
// ▐▌   ▐▌ ▐▌▐▌   ▐▌     █  ▐▛▚▖▐▌▐▌
// ▐▌   ▐▌ ▐▌▐▌▝▜▌▐▌▝▜▌  █  ▐▌ ▝▜▌▐▌▝▜▌
// ▐▙▄▄▖▝▚▄▞▘▝▚▄▞▘▝▚▄▞▘▗▄█▄▖▐▌  ▐▌▝▚▄▞▘
//

func log(ctx context.Context, level, name string, err error, attrs ...A) {

	var (
		t = time.Now()
		s = stack_backend.Get(ctx)
	)

	if s.Options.AddLogsToSpan {
		l := stack_backend.SpanLog{
			Time:  t,
			Name:  name,
			Attrs: append(attrs, Attr("level", level)),
		}
		if err != nil {
			l.Attrs = append(l.Attrs, Attr("error", err))
		}
		s.Span.OwnLogs = append(s.Span.OwnLogs, l)
	}

	if err != nil {
		s.Span.Error = err
	}

	var e = stack_backend.Event{
		Kind:  stack_backend.KindLog,
		State: s.Clone(),
		LogEvent: stack_backend.LogEvent{
			ID:       stack_backend.NewID(),
			Time:     t,
			Name:     name,
			Level:    level,
			OwnAttrs: attrs,
			Error:    err,
		},
	}

	if err != nil {
		e.Kind |= stack_backend.KindError
	}

	s.Backend.Handle(e)
}

func Log(ctx context.Context, level, name string, attrs ...A) {
	log(ctx, level, name, nil, attrs...)
}

func Debug(ctx context.Context, name string, attrs ...A) {
	log(ctx, stack_backend.LevelDebug, name, nil, attrs...)
}

func Info(ctx context.Context, name string, attrs ...A) {
	log(ctx, stack_backend.LevelInfo, name, nil, attrs...)
}

func Warn(ctx context.Context, name string, attrs ...A) {
	log(ctx, stack_backend.LevelWarn, name, nil, attrs...)
}

func Error(ctx context.Context, name string, err error, attrs ...A) error {
	log(ctx, stack_backend.LevelError, name, err, attrs...)
	return nil
}

// TODO
// Panic()

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

	var attrs []A
	for i := 0; i < val.NumField(); i++ {
		var (
			field      = typ.Field(i)
			fieldName  = field.Name
			fieldValue = val.Field(i)
		)

		if !fieldValue.CanInterface() {
			continue
		}

		if name, ok := field.Tag.Lookup("name"); ok {
			fieldName = name
		}

		attrs = append(attrs, A{
			Name:  fieldName,
			Value: fieldValue.Interface(),
		})
	}

	log(ctx, stack_backend.LevelInfo, fullName, nil, attrs...)
}

//
// ▗▄▄▄▖▗▄▄▖ ▗▄▄▖  ▗▄▖ ▗▄▄▖  ▗▄▄▖
// ▐▌   ▐▌ ▐▌▐▌ ▐▌▐▌ ▐▌▐▌ ▐▌▐▌
// ▐▛▀▀▘▐▛▀▚▖▐▛▀▚▖▐▌ ▐▌▐▛▀▚▖ ▝▀▚▖
// ▐▙▄▄▖▐▌ ▐▌▐▌ ▐▌▝▚▄▞▘▐▌ ▐▌▗▄▄▞▘
//

// TODO
func Recover(ctx context.Context, rFn func(rec any)) {
	if rec := recover(); rec != nil {
		rFn(rec)
	}
}
