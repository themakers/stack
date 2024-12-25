package stack_backend

import (
	"github.com/DataDog/gostackparse"
	"time"
)

const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"

	LevelSpan    = "span+"
	LevelSpanEnd = "span-"
)

var _ Option = Attr{}

type Attr struct {
	Name  string
	Value any
}

func (a Attr) ApplyToStack(s *Stack) {
	s.Span.Attrs = append(s.Span.Attrs, a)
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

// TODO: Review/refactor

type Event struct {
	Kind Kind

	State *Stack

	LogEvent LogEvent

	// TODO // File
	// TODO // Line
	// TODO // StackTrace []byte

	//> Metric
	// TODO
}

type LogEvent struct {
	ID ID

	Name string

	Time time.Time

	Level      string
	Error      error
	Panic      any
	IsTypedLog bool

	OwnAttrs   []Attr
	StackTrace *gostackparse.Goroutine
}
