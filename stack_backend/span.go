package stack_backend

import (
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
