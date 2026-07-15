package stack_backend

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"runtime"
	"sync"
	"time"
	"unsafe"
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

func (options Options) Environment(name string) Options {
	return append(options, OptionFunc(func(s *Stack) {
		s.Options.Environment = name
	}))
}

func (options Options) Instance(name string) Options {
	return append(options, OptionFunc(func(s *Stack) {
		s.Options.Instance = name
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
	return append(options, applyW3C(traceparent))
}

func (options Options) W3CTraceContextFromRequest(q *http.Request) Options {
	return append(options, w3cFromRequest(q))
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

func (options Options) Cancel() Options {
	return append(options, OptionFunc(func(s *Stack) {
		s.CloseContextWithSpan = true
	}))
}

//
//  ▗▄▄▖ ▗▄▖ ▗▖  ▗▖▗▄▄▄▖▗▄▄▄▖▗▖  ▗▖▗▄▄▄▖
// ▐▌   ▐▌ ▐▌▐▛▚▖▐▌  █  ▐▌    ▝▚▞▘   █
// ▐▌   ▐▌ ▐▌▐▌ ▝▜▌  █  ▐▛▀▀▘  ▐▌    █
// ▝▚▄▄▖▝▚▄▞▘▐▌  ▐▌  █  ▐▙▄▄▖▗▞▘▝▚▖  █
//

type stackCtxKey struct{}

// defaultStack is a shared template for a context without a configured Stack.
// Never mutated directly: Get returns its Clone, so mutations happen on the
// copy. Backend is noopBackend so that calls on an unconfigured ctx do not
// nil-dereference (bug: Backend used to be nil).
var defaultStack = func() *Stack {
	s := &Stack{}
	s.Options.AddLogsToSpan = true
	s.Backend = noopBackend{}
	return s
}()

func Get(ctx context.Context) *Stack {
	if s, ok := ctx.Value(stackCtxKey{}).(*Stack); ok {
		return s
	}
	return defaultStack.Clone()
}

func Put(ctx context.Context, s *Stack) context.Context {
	return context.WithValue(ctx, stackCtxKey{}, s)
}

// noopBackend is a safe default for an unconfigured context.
type noopBackend struct{}

func (noopBackend) Handle(Event)             {}
func (noopBackend) Shutdown(context.Context) {}

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
		Environment string
		Instance    string
		ScopeAttrs  []Attr
	}

	CloseContextWithSpan bool
}

// stackLocks are striped locks guarding mutations of a particular *Stack's
// fields (Span.OwnLogs, Span.Error) under concurrent logging from multiple
// goroutines sharing one context. The lock is picked by pointer hash, which
// avoids storing a mutex in the struct itself (that would trigger copylocks
// for consumers and cost an extra allocation). Index collisions are safe —
// at worst unrelated Stacks get falsely serialized, which is negligible for
// the logging path.
const stackLockStripes = 64

var stackLocks [stackLockStripes]sync.Mutex

func lockFor(s *Stack) *sync.Mutex {
	// Pointer hash: mix the bits and take the low ones.
	h := uintptr(unsafe.Pointer(s))
	h ^= h >> 11
	return &stackLocks[h&(stackLockStripes-1)]
}

// LockState/UnlockState serialize mutations of this Stack's fields. Exported
// for the calling layer (package stack), which needs to append a log/error
// atomically. The names are intentionally NOT Lock/Unlock: otherwise Stack
// would satisfy sync.Locker and go vet copylocks would flag the legitimate
// copy in Clone().
func (s *Stack) LockState()   { lockFor(s).Lock() }
func (s *Stack) UnlockState() { lockFor(s).Unlock() }

// Clone returns a copy of the Stack for a new span/scope. Slices are copied
// with a full slice expression ([:len:len]): a child append is forced to
// reallocate and cannot clobber the parent's backing array (otherwise sibling
// spans would overwrite each other's Attrs/OwnLogs). Copying happens under
// the source's lock so a concurrent log() does not race with reading the
// slice headers.
func (s *Stack) Clone() *Stack {
	mu := lockFor(s)
	mu.Lock()
	cloned := *s
	mu.Unlock()

	cloned.Span.Attrs = clip(cloned.Span.Attrs)
	cloned.Span.OwnLogs = clip(cloned.Span.OwnLogs)
	cloned.Options.ScopeAttrs = clip(cloned.Options.ScopeAttrs)

	return &cloned
}

// clip returns a slice with cap==len (or nil when empty), making subsequent
// appends copy-on-write with respect to the original backing array.
func clip[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}
	return s[:len(s):len(s)]
}

type Span struct {
	ID           ID
	ParentSpanID ID
	TraceID      TraceID

	Name string

	File string
	Line int

	Time    time.Time
	EndTime time.Time

	Error           error
	ErrorStackTrace StackTrace

	Attrs []Attr

	OwnLogs []SpanLog
}

func (s Span) IsValid() bool {
	return !s.ID.IsZero() && !s.TraceID.IsZero()
}

type SpanLog struct {
	Time  time.Time
	Name  string
	Level string
	Error error
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

// NewTraceID generates a random 128-bit trace id.
//
// It uses math/rand/v2 (ChaCha8, auto-seeded with OS entropy) rather than
// crypto/rand: tracing identifiers are correlation keys, not secrets, and do
// not require cryptographic strength (see W3C Trace Context and the reference
// otel-go SDK). In return we get no syscall per span and per-P state with no
// global lock.
func NewTraceID() (id TraceID) {
	binary.LittleEndian.PutUint64(id[0:8], rand.Uint64())
	binary.LittleEndian.PutUint64(id[8:16], rand.Uint64())
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

// NewID generates a random 64-bit span id (see NewTraceID for the rationale
// behind the generator choice).
func NewID() (id ID) {
	binary.LittleEndian.PutUint64(id[:], rand.Uint64())
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
