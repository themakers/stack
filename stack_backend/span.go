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
	var cloned *Span
	if s == nil {
		cloned = &Span{}
	} else {
		cloned = &Span{
			RootSpanID:   s.RootSpanID,
			ParentSpanID: s.ParentSpanID,
			ID:           s.ID,
			Attrs:        make([]Attr, len(s.Attrs)),
			Backend:      s.Backend,
		}
		copy(cloned.Attrs, s.Attrs)
	}

	cloned.Time = time.Now()

	if modFn != nil {
		modFn(cloned)
	}

	return cloned
}

func (s *Span) SetOrigin(traceID, parentSpanID string) {
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
}

func (s *Span) PushSpanID(newSpanID string) {
	if s.ID == "" {
		s.ID = newSpanID
	}
	if s.ParentSpanID != "" && s.RootSpanID == "" {
		s.RootSpanID = s.ParentSpanID
	}
	s.ParentSpanID = s.ID
	s.ID = newSpanID
}
