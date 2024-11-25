package span

import (
	"time"

	"github.com/rs/xid"

	"github.com/thearchitect/stack/stack_backend"
)

func GenerateID() string {
	return xid.New().String()
}

type Span struct {
	ID           string
	RootSpanID   string
	ParentSpanID string

	Name string

	Time time.Time

	Attrs []stack_backend.Attr

	Backend stack_backend.Backend
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
			Attrs:        make([]stack_backend.Attr, len(s.Attrs)),
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
