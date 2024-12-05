package stack_backend

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime"
	"time"
)

type SpanOption interface {
	ApplyToSpan(s *Span)
}

func SpanOptionFunc(fn func(s *Span)) SpanOption { return applyToSpanFunc(fn) }

var _ SpanOption = applyToSpanFunc(func(s *Span) {})

type applyToSpanFunc func(s *Span)

func (a applyToSpanFunc) ApplyToSpan(s *Span) { a(s) }

//
//  ▗▄▄▖ ▗▄▖ ▗▖  ▗▖▗▄▄▄▖▗▄▄▄▖▗▖  ▗▖▗▄▄▄▖
// ▐▌   ▐▌ ▐▌▐▛▚▖▐▌  █  ▐▌    ▝▚▞▘   █
// ▐▌   ▐▌ ▐▌▐▌ ▝▜▌  █  ▐▛▀▀▘  ▐▌    █
// ▝▚▄▄▖▝▚▄▞▘▐▌  ▐▌  █  ▐▙▄▄▖▗▞▘▝▚▖  █
//

type stackCtxKey struct{}

func Get(ctx context.Context) *Span {
	if s, ok := ctx.Value(stackCtxKey{}).(*Span); ok {
		return s
	} else {
		return &Span{}
	}
}

func Clone(ctx context.Context, modFn func(s *Span)) *Span {
	return Get(ctx).Clone(modFn)
}

func Put(ctx context.Context, s *Span) context.Context {
	return context.WithValue(ctx, stackCtxKey{}, s)
}

//
//  ▗▄▄▖▗▄▄▖  ▗▄▖ ▗▖  ▗▖
// ▐▌   ▐▌ ▐▌▐▌ ▐▌▐▛▚▖▐▌
//  ▝▀▚▖▐▛▀▘ ▐▛▀▜▌▐▌ ▝▜▌
// ▗▄▄▞▘▐▌   ▐▌ ▐▌▐▌  ▐▌
//

type Span struct {
	ID           ID
	ParentSpanID ID
	TraceID      TraceID

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

func (id TraceID) IsZero() bool {
	var id0 TraceID
	return bytes.Equal(id[:], id0[:])
}

func (id TraceID) Bytes() []byte {
	return id[:]
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

func (id ID) IsZero() bool {
	var id0 ID
	return bytes.Equal(id[:], id0[:])
}

func (id ID) Bytes() []byte {
	return id[:]
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
