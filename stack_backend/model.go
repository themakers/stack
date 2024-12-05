package stack_backend

import "time"

const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"

	LevelSpan    = "span+"
	LevelSpanEnd = "span-"
)

var _ SpanOption = Attr{}

type Attr struct {
	Name  string
	Value any
}

func (a Attr) ApplyToSpan(s *Span) {
	s.Attrs = append(s.Attrs, a)
}

type RawAttrValue string

type Kind uint8

const (
	KindSpan Kind = 1 << iota
	KindSpanEnd
	KindLog
	KindError
	//KindMetric
)

type Event struct {
	Kind Kind

	ID       ID
	ParentID ID
	TraceID  TraceID
	Name     string

	Time time.Time

	Attrs    []Attr
	OwnAttrs []Attr

	StackTrace []byte

	//> Span End
	EndTime time.Time

	//> Log
	Level      string
	Error      error
	Panic      any
	IsTypedLog bool

	//> Metric
	// TODO
}
