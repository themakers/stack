package stack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/themakers/stack/stack_backend"
	"github.com/themakers/stack/stack_backend/stack_backend_text"
)

type A = stack_backend.Attr

// Attr and F build an attribute. Generic on purpose: the value is captured
// with its static type and dispatched via a pointer type-switch, so common
// kinds (string, ints, bool, float, duration, time) are packed into
// stack_backend.Value with zero heap allocations — a plain `any` parameter
// would box every escaping non-pointer value on the caller side.
func Attr[T any](name string, value T) A {
	return F(name, value)
}

func F[T any](name string, value T) A {
	var val stack_backend.Value
	switch p := any(&value).(type) {
	case *string:
		val = stack_backend.StringValue(*p)
	case *int:
		val = stack_backend.Int64Value(int64(*p))
	case *int8:
		val = stack_backend.Int64Value(int64(*p))
	case *int16:
		val = stack_backend.Int64Value(int64(*p))
	case *int32:
		val = stack_backend.Int64Value(int64(*p))
	case *int64:
		val = stack_backend.Int64Value(*p)
	case *uint:
		val = stack_backend.Uint64Value(uint64(*p))
	case *uint8:
		val = stack_backend.Uint64Value(uint64(*p))
	case *uint16:
		val = stack_backend.Uint64Value(uint64(*p))
	case *uint32:
		val = stack_backend.Uint64Value(uint64(*p))
	case *uint64:
		val = stack_backend.Uint64Value(*p)
	case *bool:
		val = stack_backend.BoolValue(*p)
	case *float32:
		val = stack_backend.Float64Value(float64(*p))
	case *float64:
		val = stack_backend.Float64Value(*p)
	case *time.Duration:
		val = stack_backend.DurationValue(*p)
	case *time.Time:
		val = stack_backend.TimeValue(*p)
	case *stack_backend.RawAttrValue:
		val = stack_backend.RawValue(*p)
	case *stack_backend.Value:
		val = *p
	default:
		// Interfaces (incl. error), maps, structs, slices: boxed as-is.
		val = stack_backend.AnyValue(value)
	}
	return A{Name: name, Value: val}
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

// WithHostFields adds the host name as the semconv resource attribute
// host.name. Read once, at option construction: the hostname does not change
// within a process lifetime, and a failed lookup must not add an empty
// attribute pretending the host is known.
func WithHostFields() stack_backend.Option {
	host, err := os.Hostname()
	return stack_backend.OptionFunc(func(sb *stack_backend.Stack) {
		if err != nil || host == "" {
			return
		}
		sb.Options.ScopeAttrs = append(sb.Options.ScopeAttrs, F("host.name", host))
	})
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
	//
	// Kind and Links are reset for the same reason: a child of a SERVER span
	// is not itself a server, and the parent's links describe the parent's
	// causality, not this span's.
	s.Span.OwnLogs = nil
	s.Span.Error = nil
	s.Span.ErrorStackTrace = nil
	s.Span.Kind = stack_backend.SpanKindInternal
	s.Span.Links = nil

	s.Span.ParentSpanID = s.Span.ID
	s.Span.ID = stack_backend.NewID()

	// The function name is stored "raw" (without the "()" suffix): the string
	// shares memory with pclntab, no copies or concatenations. Backends append
	// the "()" suffix at render time (see stack_backend_text). This removes an
	// allocation from every span.
	s.Span.Name, s.Span.File, s.Span.Line = stack_backend.Operation(0)

	//> Apply options from arguments
	stack_backend.Options(opts).ApplyToStack(s)

	// Identity is finalized after the options: NewTrace/W3CTraceContext/TraceID
	// rewrite it, so generating ids earlier would either be wasted work or —
	// for NewTrace — leave the span with zero ids.
	if s.Span.TraceID.IsZero() {
		s.Span.TraceID = stack_backend.NewTraceID()
	}
	if s.Span.ID.IsZero() {
		s.Span.ID = stack_backend.NewID()
	}

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

// log is the shared implementation of the logging API. failSpan tells whether
// an error carried by this event marks the enclosing span as failed: a true
// failure does, a handled one (see Transient) does not.
func log(ctx context.Context, level, name string, err error, st stack_backend.StackTrace, failSpan bool, attrs ...A) {

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
	if s.Options.AddLogsToSpan || (err != nil && failSpan) {
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
		if err != nil && failSpan && s.Span.Error == nil {
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

	// KindError marks a genuine failure and is used for backend routing
	// (MuxBackendRule.Kinds), so a handled error must not raise it — see
	// Transient.
	if err != nil && failSpan {
		e.Kind |= stack_backend.KindError
	}

	s.Backend.Handle(e)
}

func Log(ctx context.Context, level, name string, attrs ...A) {
	log(ctx, level, name, nil, nil, true, attrs...)
}

func Debug(ctx context.Context, name string, attrs ...A) {
	log(ctx, stack_backend.LevelDebug, name, nil, nil, true, attrs...)
}

func Info(ctx context.Context, name string, attrs ...A) {
	log(ctx, stack_backend.LevelInfo, name, nil, nil, true, attrs...)
}

func Warn(ctx context.Context, name string, attrs ...A) {
	log(ctx, stack_backend.LevelWarn, name, nil, nil, true, attrs...)
}

func Error(ctx context.Context, name string, err error, attrs ...A) error {
	err, trace := traced(err)
	log(ctx, stack_backend.LevelError, name, err, trace, true, attrs...)
	return err
}

// Transient reports an error that has been handled — a retry, a fallback, a
// degraded path — at warn level. The error keeps full fidelity (stack trace,
// the error attribute in OTLP), but the enclosing span is NOT marked as
// failed: the operation has not failed, so painting the trace red would make
// "show me broken traces" useless.
//
// Use Error when the operation did fail. The decision belongs here, at the
// call site: no one up the stack can tell a retried attempt from a real
// failure.
func Transient(ctx context.Context, name string, err error, attrs ...A) error {
	err, trace := traced(err)
	log(ctx, stack_backend.LevelWarn, name, err, trace, false, attrs...)
	return err
}

// traced attaches a stack trace to the error, reusing the one captured at the
// original error site when it is already there.
func traced(err error) (error, stack_backend.StackTrace) {
	if err == nil {
		return nil, stack_backend.Stacktrace(1)
	} else if withTrace, ok := errors.AsType[errorWithStackTrace](err); ok {
		return err, withTrace.StackTrace()
	} else {
		trace := stack_backend.Stacktrace(1)
		return &tracedError{cause: err, trace: trace}, trace
	}
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
			Value: stack_backend.AnyValue(fieldValue.Interface()),
		})
	}

	log(ctx, stack_backend.LevelInfo, fullName, nil, nil, true, attrs...)
}

//
// ▗▄▄▄▖▗▄▄▖  ▗▄▖  ▗▄▄▖▗▄▄▄▖  ▗▄▄▖ ▗▄▖ ▗▖  ▗▖▗▄▄▄▖▗▄▄▄▖▗▖  ▗▖▗▄▄▄▖
//   █  ▐▌ ▐▌▐▌ ▐▌▐▌   ▐▌    ▐▌   ▐▌ ▐▌▐▛▚▖▐▌  █  ▐▌    ▝▚▞▘   █
//   █  ▐▛▀▚▖▐▛▀▜▌▐▌   ▐▛▀▀▘ ▐▌   ▐▌ ▐▌▐▌ ▝▜▌  █  ▐▛▀▀▘  ▐▌    █
//   █  ▐▌ ▐▌▐▌ ▐▌▝▚▄▄▖▐▙▄▄▖ ▝▚▄▄▖▝▚▄▞▘▐▌  ▐▌  █  ▐▙▄▄▖▗▞▘▝▚▖  █
//

// ExportTraceparent writes the current span into the W3C traceparent header,
// so the callee continues the same trace. Call it after stack.Span, otherwise
// the header will point at the caller's span instead of the outbound-request
// one.
//
// No-op when there is no span in the context: an invalid all-zero traceparent
// is worse than none — the receiving side would start a broken trace.
//
// connect-go exposes the outbound headers as an http.Header
// (connect.Request.Header()), so the same function covers both plain HTTP and
// connect clients.
func ExportTraceparent(ctx context.Context, h http.Header) {
	if h == nil {
		return
	}

	s := stack_backend.Get(ctx)
	tp := stack_backend.FormatW3CTraceParent(s.Span.TraceID, s.Span.ID)
	if tp == "" {
		return
	}

	h.Set(stack_backend.TraceParentHeaderName(), tp)
}

// TraceContext returns the current span's identifiers — for storing them
// alongside a queued item, so the worker that picks it up can Link back to the
// trace that produced it. Zero values mean there is no span in the context.
func TraceContext(ctx context.Context) (stack_backend.TraceID, stack_backend.ID) {
	s := stack_backend.Get(ctx)
	return s.Span.TraceID, s.Span.ID
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
