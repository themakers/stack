package stack_backend

import (
	"context"
	"runtime"
	"time"

	"github.com/rs/xid"
)

func GenerateID() string {
	return xid.New().String()
}

type SpanOption interface {
	ApplyToSpan(s *Span)
}

func SpanOptionFunc(fn func(s *Span)) SpanOption { return applyToSpanFunc(fn) }

var _ SpanOption = applyToSpanFunc(func(s *Span) {})

type applyToSpanFunc func(s *Span)

func (a applyToSpanFunc) ApplyToSpan(s *Span) { a(s) }

//
//  ▗▄▄▖ ▗▄▖ ▗▖  ▗▖▗▄▄▄▖▗▄▄▄▖▗▖  ▗▖▗▄▄▄▖
// ▐▌   ▐▌ ▐▌▐▛▚▖▐▌  █  ▐▌    ▝▚▞▘   █
// ▐▌   ▐▌ ▐▌▐▌ ▝▜▌  █  ▐▛▀▀▘  ▐▌    █
// ▝▚▄▄▖▝▚▄▞▘▐▌  ▐▌  █  ▐▙▄▄▖▗▞▘▝▚▖  █
//

type stackCtxKey struct{}

func Get(ctx context.Context) *Span {
	if s, ok := ctx.Value(stackCtxKey{}).(*Span); ok {
		return s
	} else {
		return &Span{}
	}
}

func Clone(ctx context.Context, modFn func(s *Span)) *Span {
	return Get(ctx).Clone(modFn)
}

func Put(ctx context.Context, s *Span) context.Context {
	return context.WithValue(ctx, stackCtxKey{}, s)
}

//
//  ▗▄▄▖▗▄▄▖  ▗▄▖ ▗▖  ▗▖
// ▐▌   ▐▌ ▐▌▐▌ ▐▌▐▛▚▖▐▌
//  ▝▀▚▖▐▛▀▘ ▐▛▀▜▌▐▌ ▝▜▌
// ▗▄▄▞▘▐▌   ▐▌ ▐▌▐▌  ▐▌
//

type Span struct {
	ID           string
	RootSpanID   string
	ParentSpanID string

	Name string

	Time time.Time

	Attrs []Attr

	Backend Backend
}

func (s *Span) Clone(modFn func(s *Span)) *Span {
	var cloned Span
	if s != nil {
		cloned = *s
		cloned.Attrs = make([]Attr, len(s.Attrs))
		copy(cloned.Attrs, s.Attrs)
	}

	cloned.Time = time.Now()

	if modFn != nil {
		modFn(&cloned)
	}

	return &cloned
}

//
// ▗▖ ▗▖▗▄▄▄▖▗▄▄▄▖▗▖    ▗▄▄▖
// ▐▌ ▐▌  █    █  ▐▌   ▐▌
// ▐▌ ▐▌  █    █  ▐▌    ▝▀▚▖
// ▝▚▄▞▘  █  ▗▄█▄▖▐▙▄▄▖▗▄▄▞▘
//

func Operation(skip int) (funcName string, file string, line int) {
	const (
		baseSkip = 2
		unknown  = "<unknown>"
	)

	pc, f, l, ok := runtime.Caller(baseSkip + skip)
	fn := runtime.FuncForPC(pc)
	if ok && fn != nil {
		return fn.Name(), f, l
	} else {
		return unknown, unknown, -1
	}
}
