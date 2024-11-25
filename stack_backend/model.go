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

type Attr struct {
	Name  string
	Value any
}

type Kind struct {
	Span    bool
	SpanEnd bool
	Log     bool
	Error   bool
	Metric  bool
}

type Event struct {
	Kind Kind

	ID       string
	ParentID string
	RootID   string
	Name     string

	Time time.Time

	Attrs    []Attr
	OwnAttrs []Attr

	StackTrace []byte

	//> Span End
	EndTime time.Time

	//> Log
	Level string
	Error error

	//> Metric
	// TODO
}
