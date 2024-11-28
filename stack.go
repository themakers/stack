package stack

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/themakers/stack/stack_backend"
)

type Attr = stack_backend.Attr

func New(ctx context.Context, backend stack_backend.Backend) context.Context {
	return stack_backend.Put(ctx, stack_backend.Clone(ctx, func(s *stack_backend.Span) {
		s.Backend = backend
	}))
}

//
//  ▗▄▖ ▗▄▄▖▗▄▄▄▖▗▄▄▄▖ ▗▄▖ ▗▖  ▗▖ ▗▄▄▖
// ▐▌ ▐▌▐▌ ▐▌ █    █  ▐▌ ▐▌▐▛▚▖▐▌▐▌
// ▐▌ ▐▌▐▛▀▘  █    █  ▐▌ ▐▌▐▌ ▝▜▌ ▝▀▚▖
// ▝▚▄▞▘▐▌    █  ▗▄█▄▖▝▚▄▞▘▐▌  ▐▌▗▄▄▞▘
//

func Trace(traceID, parentSpanID string) stack_backend.SpanOption {
	return stack_backend.SpanOptionFunc(func(s *stack_backend.Span) {
		if traceID != "" {
			s.RootSpanID = traceID
		}
		if parentSpanID != "" {
			s.ParentSpanID = parentSpanID
		}
		if s.RootSpanID == "" {
			s.RootSpanID = s.ParentSpanID
		}
		if s.ParentSpanID == "" {
			s.ParentSpanID = s.RootSpanID
		}
	})
}

func Name(name string) stack_backend.SpanOption {
	return stack_backend.SpanOptionFunc(func(s *stack_backend.Span) {
		s.Name = name
	})
}

func A(name string, value any) Attr {
	return Attr{Name: name, Value: value}
}

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

func Span(ctx context.Context, opts ...stack_backend.SpanOption) (context.Context, endFunc) {
	var s = stack_backend.Clone(ctx, nil)

	var newSpanID = stack_backend.GenerateID()

	{ //> Set new span id (while pushing back parent)
		if s.ID == "" {
			s.ID = newSpanID
		}
		if s.ParentSpanID != "" && s.RootSpanID == "" {
			s.RootSpanID = s.ParentSpanID
		}
		s.ParentSpanID = s.ID
		s.ID = newSpanID
	}

	s.Name, _, _ = stack_backend.Operation(0)
	s.Name = fmt.Sprint(s.Name, "()") // ???

	for _, o := range opts {
		o.ApplyToSpan(s)
	}

	ctx = stack_backend.Put(ctx, s)

	s.Backend.Handle(stack_backend.Event{
		Kind:     stack_backend.KindSpan,
		ID:       s.ID,
		ParentID: s.ParentSpanID,
		RootID:   s.RootSpanID,
		Name:     s.Name,
		Time:     s.Time,
		Attrs:    s.Attrs,
	})

	return ctx, func() {
		s.Backend.Handle(stack_backend.Event{
			Kind:     stack_backend.KindSpanEnd,
			ID:       s.ID,
			ParentID: s.ParentSpanID,
			RootID:   s.RootSpanID,
			Name:     s.Name,
			Time:     s.Time,
			Attrs:    s.Attrs,
			EndTime:  time.Now(),
		})
	}
}

//
// ▗▖    ▗▄▖  ▗▄▄▖ ▗▄▄▖▗▄▄▄▖▗▖  ▗▖ ▗▄▄▖
// ▐▌   ▐▌ ▐▌▐▌   ▐▌     █  ▐▛▚▖▐▌▐▌
// ▐▌   ▐▌ ▐▌▐▌▝▜▌▐▌▝▜▌  █  ▐▌ ▝▜▌▐▌▝▜▌
// ▐▙▄▄▖▝▚▄▞▘▝▚▄▞▘▝▚▄▞▘▗▄█▄▖▐▌  ▐▌▝▚▄▞▘
//

func log(ctx context.Context, level, name string, err error, attrs ...Attr) {
	var s = stack_backend.Get(ctx)

	var e = stack_backend.Event{
		Kind:     stack_backend.KindLog,
		ID:       stack_backend.GenerateID(),
		ParentID: s.ID,
		RootID:   s.RootSpanID,
		Attrs:    s.Attrs,
		Time:     time.Now(),
		Name:     name,
		Level:    level,
		OwnAttrs: attrs,
		Error:    err,
	}

	if err != nil {
		e.Kind |= stack_backend.KindError
	}

	s.Backend.Handle(e)
}

func Log(ctx context.Context, level, name string, attrs ...Attr) {
	log(ctx, level, name, nil, attrs...)
}

func Debug(ctx context.Context, name string, attrs ...Attr) {
	log(ctx, stack_backend.LevelDebug, name, nil, attrs...)
}

func Info(ctx context.Context, name string, attrs ...Attr) {
	log(ctx, stack_backend.LevelInfo, name, nil, attrs...)
}

func Warn(ctx context.Context, name string, attrs ...Attr) {
	log(ctx, stack_backend.LevelWarn, name, nil, attrs...)
}

func Error(ctx context.Context, name string, err error, attrs ...Attr) error {
	log(ctx, stack_backend.LevelError, name, err, attrs...)
	return nil
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

	log(ctx, stack_backend.LevelInfo, fullName, nil, attrs...)
}

//
// ▗▄▄▄▖▗▄▄▖ ▗▄▄▖  ▗▄▖ ▗▄▄▖  ▗▄▄▖
// ▐▌   ▐▌ ▐▌▐▌ ▐▌▐▌ ▐▌▐▌ ▐▌▐▌
// ▐▛▀▀▘▐▛▀▚▖▐▛▀▚▖▐▌ ▐▌▐▛▀▚▖ ▝▀▚▖
// ▐▙▄▄▖▐▌ ▐▌▐▌ ▐▌▝▚▄▞▘▐▌ ▐▌▗▄▄▞▘
//

func Recover(ctx context.Context, rFn func(rec any)) {
	if rec := recover(); rec != nil {
		rFn(rec)
	}
}
