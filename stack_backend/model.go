package stack_backend

import (
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

// SpanKind is the span's role in the trace (OTel semconv). The zero value is
// Internal — both the overwhelming majority of spans and the default mandated
// by the specification, so we never emit OTLP's UNSPECIFIED.
type SpanKind uint8

const (
	SpanKindInternal SpanKind = iota
	// SpanKindServer — handling an inbound request (the remote peer waits).
	SpanKindServer
	// SpanKindClient — an outbound request (we wait for the remote peer).
	SpanKindClient
	// SpanKindProducer — an asynchronous message is published; the consumer's
	// work is not covered by this span.
	SpanKindProducer
	// SpanKindConsumer — processing an asynchronous message published earlier.
	SpanKindConsumer
)

func (k SpanKind) String() string {
	switch k {
	case SpanKindServer:
		return "server"
	case SpanKindClient:
		return "client"
	case SpanKindProducer:
		return "producer"
	case SpanKindConsumer:
		return "consumer"
	default:
		return "internal"
	}
}

// Link relates this span to a span in another trace. Unlike parent-child, a
// link carries no timing or lifetime relationship: use it for causality that
// crosses trace boundaries (a queued job and the request that enqueued it, a
// batch and the messages it covers).
type Link struct {
	TraceID TraceID
	SpanID  ID
	Attrs   []Attr
}

var _ Option = Attr{}

type Attr struct {
	Name  string
	Value Value
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

	File string
	Line int

	Time time.Time

	Level      string
	Error      error
	Panic      any
	IsTypedLog bool

	OwnAttrs   []Attr
	StackTrace StackTrace
}
