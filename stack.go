package stack

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/DataDog/gostackparse"

	"github.com/themakers/stack/stack_backend"
	"github.com/themakers/stack/stack_backend/stack_backend_text"
)

type A = stack_backend.Attr

func Attr(name string, value any) A {
	return A{Name: name, Value: value}
}

func F(name string, value any) A {
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

func Op() op {
	var name, _, _ = stack_backend.Operation(0)
	return op(name)
}

var _ stack_backend.Option = op("")

type op string

func (o op) ApplyToStack(s *stack_backend.Stack) {
	s.Span.Name = string(o)
}

func With() stack_backend.Options {
	return stack_backend.Options{}
}

func Cancel() stack_backend.Options {
	return With().Cancel()
}

func Default(ctx context.Context) context.Context {
	return With().Backend(stack_backend_text.New()).Apply(ctx)
}

func WithVCSFields() stack_backend.Option {
	info, ok := debug.ReadBuildInfo()
	return stack_backend.OptionFunc(func(sb *stack_backend.Stack) {
		if ok {
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs":
					//s.Value
				case "vcs.revision":
					sb.Options.ScopeAttrs = append(sb.Options.ScopeAttrs, F("vcs.revision", s.Value))
				case "vcs.time":
					// RFC3339 (e.g., "2024-11-01T12:34:56Z")
					if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
						sb.Options.ScopeAttrs = append(sb.Options.ScopeAttrs, F("vcs.time", t))
					}
				case "vcs.modified":
					//sb.Span.Attrs = append(sb.Span.Attrs, ...)
					sb.Options.ScopeAttrs = append(sb.Options.ScopeAttrs, F("vcs.modified", s.Value))
				}
			}
		}
	})
}

//
//  ▗▄▄▖▗▄▄▖  ▗▄▖ ▗▖  ▗▖
// ▐▌   ▐▌ ▐▌▐▌ ▐▌▐▛▚▖▐▌
//  ▝▀▚▖▐▛▀▘ ▐▛▀▜▌▐▌ ▝▜▌
// ▗▄▄▞▘▐▌   ▐▌ ▐▌▐▌  ▐▌
//

type endFunc func(cause ...error)

func Span(ctx context.Context, opts ...stack_backend.Option) (context.Context, endFunc) {
	var s = stack_backend.Get(ctx).Clone()

	s.Span.Time = time.Now()

	// A new span must not inherit parent state specific to a particular span:
	// logs, the error, and its stack trace. Clone makes a shallow copy, and
	// these fields belong to this span, not to the chain. OwnLogs = nil (not
	// an empty slice) — the first append allocates lazily.
	s.Span.OwnLogs = nil
	s.Span.Error = nil
	s.Span.ErrorStackTrace = nil

	if s.Span.TraceID.IsZero() {
		s.Span.TraceID = stack_backend.NewTraceID()
	}

	s.Span.ParentSpanID = s.Span.ID
	s.Span.ID = stack_backend.NewID()

	// The function name is stored "raw" (without the "()" suffix): the string
	// shares memory with pclntab, no copies or concatenations. Backends append
	// the "()" suffix at render time (see stack_backend_text). This removes an
	// allocation from every span.
	s.Span.Name, s.Span.File, s.Span.Line = stack_backend.Operation(0)

	//> Apply options from arguments
	stack_backend.Options(opts).ApplyToStack(s)

	var cancel context.CancelCauseFunc
	if s.CloseContextWithSpan {
		ctx, cancel = context.WithCancelCause(ctx)
	}

	ctx = stack_backend.Put(ctx, s)

	s.Backend.Handle(stack_backend.Event{
		Kind:  stack_backend.KindSpan,
		State: s,
	})

	var ended atomic.Bool
	return ctx, func(cause ...error) {
		// Idempotency: a repeated done() is a no-op, no second KindSpanEnd.
		if !ended.CompareAndSwap(false, true) {
			return
		}

		var cause0 error
		if len(cause) > 0 {
			cause0 = cause[0]
		}

		if cancel != nil {
			cancel(cause0)
		}

		// Mutations go under the lock — a concurrent log() may be reading the
		// Stack in Clone(). done(err) marks the span as failed unless an error
		// has already been set (via stack.Error inside the span).
		s.LockState()
		if cause0 != nil && s.Span.Error == nil {
			s.Span.Error = cause0
		}
		s.Span.EndTime = time.Now()
		s.UnlockState()

		// The backend gets a snapshot (Clone copies under the lock): if another
		// goroutine keeps logging into this ctx during done(), the backend must
		// not read OwnLogs/Error concurrently with their mutation.
		s.Backend.Handle(stack_backend.Event{
			Kind:  stack_backend.KindSpanEnd,
			State: s.Clone(),
		})
	}
}

//
// ▗▖    ▗▄▖  ▗▄▄▖ ▗▄▄▖▗▄▄▄▖▗▖  ▗▖ ▗▄▄▖
// ▐▌   ▐▌ ▐▌▐▌   ▐▌     █  ▐▛▚▖▐▌▐▌
// ▐▌   ▐▌ ▐▌▐▌▝▜▌▐▌▝▜▌  █  ▐▌ ▝▜▌▐▌▝▜▌
// ▐▙▄▄▖▝▚▄▞▘▝▚▄▞▘▝▚▄▞▘▗▄█▄▖▐▌  ▐▌▝▚▄▞▘
//

func log(ctx context.Context, level, name string, err error, st *gostackparse.Goroutine, attrs ...A) {

	var (
		t             = time.Now()
		s             = stack_backend.Get(ctx)
		_, file, line = stack_backend.Operation(1)
	)

	if level == stack_backend.LevelError && err == nil {
		err = errors.New(name)
	}

	// Serialize mutations of the shared *Stack (OwnLogs, Error): the same ctx
	// may be used from multiple goroutines. Level/Error are stored as SpanLog
	// fields rather than appended to attrs — this removes the variadic attrs
	// slice reallocation on every log.
	if s.Options.AddLogsToSpan || (err != nil) {
		s.LockState()
		if s.Options.AddLogsToSpan {
			s.Span.OwnLogs = append(s.Span.OwnLogs, stack_backend.SpanLog{
				Time:  t,
				Name:  name,
				Level: level,
				Error: err,
				Attrs: attrs,
			})
		}
		if err != nil && s.Span.Error == nil {
			s.Span.Error = err
			s.Span.ErrorStackTrace = st
		}
		s.UnlockState()
	}

	var e = stack_backend.Event{
		Kind:  stack_backend.KindLog,
		State: s.Clone(),
		LogEvent: stack_backend.LogEvent{
			ID:         stack_backend.NewID(),
			Time:       t,
			Name:       name,
			Level:      level,
			OwnAttrs:   attrs,
			Error:      err,
			StackTrace: st,
			File:       file,
			Line:       line,
		},
	}

	if err != nil {
		e.Kind |= stack_backend.KindError
	}

	s.Backend.Handle(e)
}

func Log(ctx context.Context, level, name string, attrs ...A) {
	log(ctx, level, name, nil, nil, attrs...)
}

func Debug(ctx context.Context, name string, attrs ...A) {
	log(ctx, stack_backend.LevelDebug, name, nil, nil, attrs...)
}

func Info(ctx context.Context, name string, attrs ...A) {
	log(ctx, stack_backend.LevelInfo, name, nil, nil, attrs...)
}

func Warn(ctx context.Context, name string, attrs ...A) {
	log(ctx, stack_backend.LevelWarn, name, nil, nil, attrs...)
}

func Error(ctx context.Context, name string, err error, attrs ...A) error {
	log(ctx, stack_backend.LevelError, name, err, stack_backend.Stacktrace(0), attrs...)
	return err
}

func TLog(ctx context.Context, typed any) {
	// The logger must not panic: report invalid input with a Warn log and return.
	val := reflect.ValueOf(typed)
	if !val.IsValid() {
		Warn(ctx, "stack.TLog: input must be a struct or pointer to a struct", F("type", "nil"))
		return
	}

	typ := val.Type()

	for typ.Kind() == reflect.Ptr {
		if val.IsNil() {
			Warn(ctx, "stack.TLog: input must be a struct or pointer to a struct", F("type", typ.String()))
			return
		}
		val = val.Elem()
		typ = val.Type()
	}

	if typ.Kind() != reflect.Struct {
		Warn(ctx, "stack.TLog: input must be a struct or pointer to a struct", F("type", typ.String()))
		return
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

	log(ctx, stack_backend.LevelInfo, fullName, nil, nil, attrs...)
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
