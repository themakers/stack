package stack_backend

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DataDog/gostackparse"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"
)

type Option interface {
	ApplyToStack(s *Stack)
}

var _ Option = OptionFunc(func(s *Stack) {})

type OptionFunc func(s *Stack)

func (a OptionFunc) ApplyToStack(s *Stack) { a(s) }

//
//  ▗▄▖ ▗▄▄▖▗▄▄▄▖▗▄▄▄▖ ▗▄▖ ▗▖  ▗▖ ▗▄▄▖
// ▐▌ ▐▌▐▌ ▐▌ █    █  ▐▌ ▐▌▐▛▚▖▐▌▐▌
// ▐▌ ▐▌▐▛▀▘  █    █  ▐▌ ▐▌▐▌ ▝▜▌ ▝▀▚▖
// ▝▚▄▞▘▐▌    █  ▗▄█▄▖▝▚▄▞▘▐▌  ▐▌▗▄▄▞▘
//

var _ Option = Options{}

type Options []Option

func (options Options) Option(option Option) Options {
	if option != nil {
		return append(options, option)
	} else {
		return options
	}
}

func (options Options) Backend(backend Backend) Options {
	return append(options, OptionFunc(func(s *Stack) {
		s.Backend = backend
	}))
}

func (options Options) ServiceName(name string) Options {
	return append(options, OptionFunc(func(s *Stack) {
		s.Options.ServiceName = name
	}))
}

func (options Options) ScopeAttrs(attrs ...Attr) Options {
	return append(options, OptionFunc(func(s *Stack) {
		s.Options.ScopeAttrs = append(s.Options.ScopeAttrs, attrs...)
	}))
}

func (options Options) Attrs(attrs ...Attr) Options {
	return append(options, OptionFunc(func(s *Stack) {
		s.Span.Attrs = append(s.Span.Attrs, attrs...)
	}))
}

func (options Options) EmbedLogsIntoSpans(embed bool) Options {
	return append(options, OptionFunc(func(s *Stack) {
		s.Options.AddLogsToSpan = embed
	}))
}

func (options Options) Name(name string) Options {
	return append(options, OptionFunc(func(s *Stack) {
		s.Span.Name = name
	}))
}

func (options Options) TraceID(traceID []byte) Options {
	return append(options, OptionFunc(func(s *Stack) {
		if !TraceID(traceID).IsZero() {
			s.Span.TraceID = TraceID(traceID)
		}
	}))
}

func (options Options) ParentSpanID(id []byte) Options {
	return append(options, OptionFunc(func(s *Stack) {
		if !ID(id).IsZero() {
			s.Span.ParentSpanID = ID(id)
		}
	}))
}

func (options Options) W3CTraceContext(traceparent, tracestate string) Options {
	return append(options, OptionFunc(func(s *Stack) {
	}))
}

func (options Options) W3CTraceContextFromRequest(q *http.Request) Options {
	return append(options, OptionFunc(func(s *Stack) {
	}))
}

func (options Options) Apply(ctx context.Context) context.Context {
	s := Get(ctx).Clone()
	options.ApplyToStack(s)
	return Put(ctx, s)
}

func (options Options) ApplyToStack(s *Stack) {
	for _, oFn := range options {
		if oFn != nil {
			oFn.ApplyToStack(s)
		}
	}
}

//
//  ▗▄▄▖ ▗▄▖ ▗▖  ▗▖▗▄▄▄▖▗▄▄▄▖▗▖  ▗▖▗▄▄▄▖
// ▐▌   ▐▌ ▐▌▐▛▚▖▐▌  █  ▐▌    ▝▚▞▘   █
// ▐▌   ▐▌ ▐▌▐▌ ▝▜▌  █  ▐▛▀▀▘  ▐▌    █
// ▝▚▄▄▖▝▚▄▞▘▐▌  ▐▌  █  ▐▙▄▄▖▗▞▘▝▚▖  █
//

type stackCtxKey struct{}

func Get(ctx context.Context) *Stack {
	if s, ok := ctx.Value(stackCtxKey{}).(*Stack); ok {
		return s
	} else {
		s := &Stack{}
		s.Options.AddLogsToSpan = true
		return s
	}
}

func Put(ctx context.Context, s *Stack) context.Context {
	return context.WithValue(ctx, stackCtxKey{}, s)
}

//
//  ▗▄▄▖▗▄▄▖  ▗▄▖ ▗▖  ▗▖
// ▐▌   ▐▌ ▐▌▐▌ ▐▌▐▛▚▖▐▌
//  ▝▀▚▖▐▛▀▘ ▐▛▀▜▌▐▌ ▝▜▌
// ▗▄▄▞▘▐▌   ▐▌ ▐▌▐▌  ▐▌
//

type Stack struct {
	Span Span

	Backend Backend

	Options struct {
		AttrPrefix    string
		AddLogsToSpan bool

		ServiceName string
		ScopeAttrs  []Attr
	}
}

func (s *Stack) Clone() *Stack {
	cloned := *s
	return &cloned
}

type Span struct {
	ID           ID
	ParentSpanID ID
	TraceID      TraceID

	Name string

	Time    time.Time
	EndTime time.Time

	Error           error
	ErrorStackTrace *gostackparse.Goroutine

	Attrs []Attr

	OwnLogs []SpanLog
}

func (s Span) IsValid() bool {
	return !s.ID.IsZero() && !s.TraceID.IsZero()
}

type SpanLog struct {
	Time  time.Time
	Name  string
	Attrs []Attr
}

//
// ▗▖ ▗▖▗▄▄▄▖▗▄▄▄▖▗▖    ▗▄▄▖
// ▐▌ ▐▌  █    █  ▐▌   ▐▌
// ▐▌ ▐▌  █    █  ▐▌    ▝▀▚▖
// ▝▚▄▞▘  █  ▗▄█▄▖▐▙▄▄▖▗▄▄▞▘
//

var _ json.Marshaler = NewTraceID()

type TraceID [16]byte

func NewTraceID() (id TraceID) {
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return id
}

func TraceIDFromString(hx string) (id TraceID, err error) {
	if v, err := hex.DecodeString(hx); err != nil {
		return id, err
	} else if len(v) != len(id) {
		return id, errors.New("malformed trace id")
	} else {
		copy(id[:], v)
		return id, err
	}
}

func (id TraceID) IsZero() bool {
	var id0 TraceID
	return bytes.Equal(id[:], id0[:])
}

func (id TraceID) Bytes() []byte {
	if id.IsZero() {
		return nil
	} else {
		return id[:]
	}
}

func (id TraceID) String() string {
	return hex.EncodeToString(id[:])
}

func (id TraceID) MarshalJSON() ([]byte, error) {
	if id.IsZero() {
		return []byte("null"), nil
	} else {
		return []byte(fmt.Sprintf(`"%s"`, id)), nil
	}
}

var _ json.Marshaler = NewID()

type ID [8]byte

func NewID() (id ID) {
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return id
}

func IDFromString(hx string) (id ID, err error) {
	if v, err := hex.DecodeString(hx); err != nil {
		return id, err
	} else if len(v) != len(id) {
		return id, errors.New("malformed id")
	} else {
		copy(id[:], v)
		return id, err
	}
}

func (id ID) IsZero() bool {
	var id0 ID
	return bytes.Equal(id[:], id0[:])
}

func (id ID) Bytes() []byte {
	if id.IsZero() {
		return nil
	} else {
		return id[:]
	}
}

func (id ID) String() string {
	return hex.EncodeToString(id[:])
}

func (id ID) MarshalJSON() ([]byte, error) {
	if id.IsZero() {
		return []byte("null"), nil
	} else {
		return []byte(fmt.Sprintf(`"%s"`, id)), nil
	}
}

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

func Stacktrace(skip int) *gostackparse.Goroutine {
	const baseSkip = 2

	s := debug.Stack()

	goroutines, errs := gostackparse.Parse(bytes.NewReader(s))

	if len(errs) > 0 {
		println("malformed stacktrace:", string(s))
	}

	goroutine := goroutines[0]

	goroutine.Stack = goroutine.Stack[baseSkip+skip:]

	return goroutine
}
