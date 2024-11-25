package span

import (
	"github.com/thearchitect/stack/stack_backend"
	"time"
)

func (s *Span) FireSpan() {
	s.Backend.Handle(stack_backend.Event{
		ID:       s.ID,
		ParentID: s.ParentSpanID,
		RootID:   s.RootSpanID,
		Name:     s.Name,
		Time:     s.Time,
		Attrs:    s.Attrs,
	})
}

func (s *Span) FireSpanEnd() {
	s.Backend.Handle(stack_backend.Event{
		ID:       s.ID,
		ParentID: s.ParentSpanID,
		RootID:   s.RootSpanID,
		Name:     s.Name,
		Time:     s.Time,
		Attrs:    s.Attrs,
		EndTime:  time.Now(),
	})
}

func (s *Span) FireLog(name string, level string, err error, ownAttrs []stack_backend.Attr) {
	s.Backend.Handle(stack_backend.Event{
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
