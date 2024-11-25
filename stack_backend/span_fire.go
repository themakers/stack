package stack_backend

import (
	"time"
)

func (s *Span) FireSpan() {
	s.Backend.Handle(Event{
		Kind: Kind{
			Span: true,
		},
		ID:       s.ID,
		ParentID: s.ParentSpanID,
		RootID:   s.RootSpanID,
		Name:     s.Name,
		Time:     s.Time,
		Attrs:    s.Attrs,
	})
}

func (s *Span) FireSpanEnd() {
	s.Backend.Handle(Event{
		Kind: Kind{
			SpanEnd: true,
		},
		ID:       s.ID,
		ParentID: s.ParentSpanID,
		RootID:   s.RootSpanID,
		Name:     s.Name,
		Time:     s.Time,
		Attrs:    s.Attrs,
		EndTime:  time.Now(),
	})
}

func (s *Span) FireLog(name string, level string, err error, ownAttrs []Attr) {
	s.Backend.Handle(Event{
		Kind: Kind{
			Log: true,
		},
		ID:       GenerateID(),
		ParentID: s.ID,
		RootID:   s.RootSpanID,
		Attrs:    s.Attrs,
		Time:     time.Now(),
		Name:     name,
		Level:    level,
		OwnAttrs: ownAttrs,
		Error:    err,
	})
}
