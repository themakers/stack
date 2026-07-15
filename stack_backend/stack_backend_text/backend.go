package stack_backend_text

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/themakers/stack/stack_backend"
)

const timeFormat = "2006-01-02 15:04:05.000000000"

var _ stack_backend.Backend = Backend{}

type Backend struct {
	w io.Writer
}

func New() stack_backend.Backend {
	return Backend{w: os.Stdout}
}

// NewWithWriter creates a text backend writing to an arbitrary io.Writer.
// Needed for tests and benchmarks (writing to io.Discard) and for redirecting
// output to a file/buffer.
func NewWithWriter(w io.Writer) stack_backend.Backend {
	return Backend{w: w}
}

// writer returns the target io.Writer, substituting os.Stdout for the zero
// value (backward compatibility with a Backend{} created without the
// constructor).
func (b Backend) writer() io.Writer {
	if b.w != nil {
		return b.w
	}
	return os.Stdout
}

// Prebuilt colors: color.New used to be called 6-8 times per event, costing
// as many allocations. color.Color objects are immutable, so they can live as
// package variables and be reused.
var (
	colTime       = color.New()
	colFileLine   = color.New()
	colNested     = color.New().AddRGB(128, 128, 128)
	colOwnAttrs   = color.New(color.FgHiWhite)
	colName       = color.New(color.FgMagenta)
	colError      = color.New(color.FgRed, color.BgWhite)
	colLvlSpan    = color.New(color.FgWhite)
	colLvlSpanEnd = color.New(color.FgHiWhite)
	colLvlDebug   = color.New(color.FgBlack, color.BgHiWhite)
	colLvlInfo    = color.New(color.FgBlack, color.BgWhite)
	colLvlWarn    = color.New(color.FgBlack, color.BgYellow)
	colLvlError   = color.New(color.FgBlack, color.BgRed)
	colLvlDefault = color.New(color.FgBlue, color.BgWhite)
)

// Precomputed color escape sequences (prefix + suffix). We write them into
// the buffer directly instead of color.Sprint* — the latter allocates an
// intermediate string per field. With colors disabled (color.NoColor),
// prefix/suffix are empty and the output is clean.
type colorCode struct {
	prefix []byte
	suffix []byte
}

func makeColorCode(c *color.Color) colorCode {
	s := c.Sprint("\x00")
	before, after, found := strings.Cut(s, "\x00")
	if !found {
		return colorCode{}
	}
	return colorCode{prefix: []byte(before), suffix: []byte(after)}
}

var (
	ccTime       = makeColorCode(colTime)
	ccFileLine   = makeColorCode(colFileLine)
	ccNested     = makeColorCode(colNested)
	ccOwnAttrs   = makeColorCode(colOwnAttrs)
	ccName       = makeColorCode(colName)
	ccError      = makeColorCode(colError)
	ccLvlSpan    = makeColorCode(colLvlSpan)
	ccLvlSpanEnd = makeColorCode(colLvlSpanEnd)
	ccLvlDebug   = makeColorCode(colLvlDebug)
	ccLvlInfo    = makeColorCode(colLvlInfo)
	ccLvlWarn    = makeColorCode(colLvlWarn)
	ccLvlError   = makeColorCode(colLvlError)
	ccLvlDefault = makeColorCode(colLvlDefault)
)

// Buffer pool: removes the bytes.Buffer allocation per event; the buffer
// warms up to a stable size.
var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// record is a snapshot of fields for rendering. Lives on Handle's stack, does
// not escape to the heap.
type record struct {
	Time     time.Time
	Level    string
	LevelCC  colorCode
	File     string
	Line     int
	Name     string
	NameSfx  bool // append "()" to the name (spans store the name without the suffix)
	Error    string
	Duration time.Duration

	OwnAttrs    []stack_backend.Attr
	NestedAttrs []stack_backend.Attr

	StackTrace stack_backend.StackTrace
}

func (b Backend) write(w io.Writer, r record) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	// time
	buf.Write(ccTime.prefix)
	buf.Write(r.Time.AppendFormat(buf.AvailableBuffer(), timeFormat))
	buf.Write(ccTime.suffix)

	// level
	buf.WriteByte(' ')
	buf.Write(r.LevelCC.prefix)
	buf.WriteByte(' ')
	writePadded(buf, r.Level, 5)
	buf.WriteByte(' ')
	buf.Write(r.LevelCC.suffix)

	// file:line
	if len(r.File) > 0 {
		buf.WriteByte(' ')
		buf.WriteByte(' ')
		buf.Write(ccFileLine.prefix)
		buf.WriteString(filepath.Base(r.File))
		buf.WriteByte(':')
		buf.Write(strconv.AppendInt(buf.AvailableBuffer(), int64(r.Line), 10))
		buf.Write(ccFileLine.suffix)
	}

	// name
	buf.WriteByte(' ')
	buf.Write(ccName.prefix)
	buf.WriteString(r.Name)
	if r.NameSfx {
		buf.WriteString("()")
	}
	buf.Write(ccName.suffix)

	// duration
	if r.Duration != 0 {
		buf.WriteByte(' ')
		buf.Write(ccOwnAttrs.prefix)
		buf.WriteString(r.Duration.String())
		buf.Write(ccOwnAttrs.suffix)
	}

	// error
	if r.Error != "" {
		buf.WriteByte(' ')
		buf.Write(ccError.prefix)
		buf.WriteString(r.Error)
		buf.Write(ccError.suffix)
	}

	// own attrs
	writeAttrs(buf, r.OwnAttrs, ccOwnAttrs)

	if len(r.OwnAttrs) > 0 && len(r.NestedAttrs) > 0 {
		buf.Write(ccNested.prefix)
		buf.WriteByte(',')
		buf.Write(ccNested.suffix)
	}

	// nested attrs
	writeAttrs(buf, r.NestedAttrs, ccNested)

	buf.WriteByte('\n')

	if r.StackTrace != nil {
		writeStackTrace(buf, r.StackTrace)
	}

	// The single write point. A write error must not panic (EPIPE on a closed
	// stdout must not crash the service) — silently ignored at the Handle level.
	_, err := buf.WriteTo(w)
	return err
}

func writeAttrs(buf *bytes.Buffer, attrs []stack_backend.Attr, cc colorCode) {
	for i, f := range attrs {
		buf.WriteByte(' ')
		buf.Write(cc.prefix)
		buf.WriteString(f.Name)
		buf.WriteByte('=')
		appendValue(buf, f.Value)
		buf.Write(cc.suffix)
		if i < len(attrs)-1 {
			buf.WriteByte(',')
		}
	}
}

// writeStackTrace renders the lazily-resolved trace. Resolution happens here,
// at render time, via the core's StackTrace.Frames iterator — frames are
// written straight into the pooled buffer without intermediate slices.
func writeStackTrace(buf *bytes.Buffer, st stack_backend.StackTrace) {
	st.Frames(func(frm stack_backend.Frame) bool {
		buf.WriteString("     ••• ")
		buf.WriteString(frm.Function)
		buf.WriteString("\n                 ")

		file := frm.File
		if idx := nthSeparatorFromEnd(file, 5); idx >= 0 {
			file = file[idx+1:]
		}
		buf.WriteString(file)
		buf.WriteByte(':')
		buf.Write(strconv.AppendInt(buf.AvailableBuffer(), int64(frm.Line), 10))
		buf.WriteString(" ••• \n")
		return true
	})
}

// nthSeparatorFromEnd returns the index of the n-th path separator counting
// from the end, or -1 (used to shorten file paths to their last n segments
// without allocating, unlike strings.Split+filepath.Join).
func nthSeparatorFromEnd(s string, n int) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == filepath.Separator {
			n--
			if n == 0 {
				return i
			}
		}
	}
	return -1
}

// writePadded writes s right-padded with spaces to width (equivalent of %-5s).
func writePadded(buf *bytes.Buffer, s string, width int) {
	buf.WriteString(s)
	for i := len(s); i < width; i++ {
		buf.WriteByte(' ')
	}
}

// appendValue writes an attribute value into the buffer without intermediate
// strings. The Value union carries its kind — dispatching on it is a plain
// integer switch, cheaper than an interface type-switch. Reflective
// json.Marshal is reserved for rare complex values (map/struct/slice).
// ValueKindRaw is printed as-is.
func appendValue(buf *bytes.Buffer, v stack_backend.Value) {
	switch v.Kind() {
	case stack_backend.ValueKindRaw:
		buf.WriteString(v.String())
	case stack_backend.ValueKindString:
		appendJSONString(buf, v.String())
	case stack_backend.ValueKindBool:
		buf.Write(strconv.AppendBool(buf.AvailableBuffer(), v.Bool()))
	case stack_backend.ValueKindInt64:
		buf.Write(strconv.AppendInt(buf.AvailableBuffer(), v.Int64(), 10))
	case stack_backend.ValueKindUint64:
		buf.Write(strconv.AppendUint(buf.AvailableBuffer(), v.Uint64(), 10))
	case stack_backend.ValueKindFloat64:
		buf.Write(strconv.AppendFloat(buf.AvailableBuffer(), v.Float64(), 'g', -1, 64))
	case stack_backend.ValueKindDuration:
		buf.WriteString(v.Duration().String())
	case stack_backend.ValueKindTime:
		buf.Write(v.Time().AppendFormat(buf.AvailableBuffer(), time.RFC3339Nano))
	case stack_backend.ValueKindError:
		if err := v.Error(); err != nil {
			appendJSONString(buf, err.Error())
		} else {
			buf.WriteString("null")
		}
	default:
		appendAnyValue(buf, v.Any())
	}
}

// appendAnyValue is the cold path for ValueKindAny payloads.
func appendAnyValue(buf *bytes.Buffer, v any) {
	switch val := v.(type) {
	case []byte:
		appendJSONString(buf, string(val))
	case nil:
		buf.WriteString("null")
	default:
		// Cold path: a complex value. json.Marshal without Indent, written
		// directly into the buffer. A marshal error does not panic — placeholder.
		if data, err := json.Marshal(v); err != nil {
			buf.WriteString("<attr marshal error>")
		} else {
			buf.Write(data)
		}
	}
}

// appendJSONString writes a string in JSON quotes. The fast path without
// json.Marshal is for strings with no characters requiring escapes; valid
// UTF-8 (including Cyrillic) is written raw: JSON strings are UTF-8, escaping
// is only mandatory for control characters, the quote, and the backslash.
func appendJSONString(buf *bytes.Buffer, s string) {
	if !needsJSONEscape(s) {
		buf.WriteByte('"')
		buf.WriteString(s)
		buf.WriteByte('"')
		return
	}
	if data, err := json.Marshal(s); err != nil {
		buf.WriteString("<attr marshal error>")
	} else {
		buf.Write(data)
	}
}

func needsJSONEscape(s string) bool {
	hasHigh := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c == '"' || c == '\\' {
			return true
		}
		if c >= 0x80 {
			hasHigh = true
		}
	}
	// Non-ASCII without special characters: only valid UTF-8 may go raw —
	// broken bytes are passed to json.Marshal (it replaces them with U+FFFD).
	return hasHigh && !utf8.ValidString(s)
}

func (b Backend) Handle(e stack_backend.Event) {
	var r record

	if e.Kind&stack_backend.KindSpan != 0 {
		r.Name = e.State.Span.Name
		r.NameSfx = true
		r.Level = stack_backend.LevelSpan
		r.LevelCC = ccLvlSpan
		r.File = e.State.Span.File
		r.Line = e.State.Span.Line
		r.Time = e.State.Span.Time
		r.OwnAttrs = e.State.Span.Attrs
	} else if e.Kind&stack_backend.KindSpanEnd != 0 {
		r.Name = e.State.Span.Name
		r.NameSfx = true
		r.Level = stack_backend.LevelSpanEnd
		r.LevelCC = ccLvlSpanEnd
		r.Time = e.State.Span.EndTime
		r.Duration = e.State.Span.EndTime.Sub(e.State.Span.Time)
		r.OwnAttrs = e.State.Span.Attrs
		if e.State.Span.Error != nil {
			r.Error = e.State.Span.Error.Error()
		}
	} else if e.Kind&stack_backend.KindLog != 0 {
		r.NestedAttrs = e.State.Span.Attrs
		r.OwnAttrs = e.LogEvent.OwnAttrs
		r.Name = e.LogEvent.Name
		r.Level = e.LogEvent.Level
		r.Time = e.LogEvent.Time
		r.File = e.LogEvent.File
		r.Line = e.LogEvent.Line
		if e.LogEvent.Error != nil {
			r.Error = e.LogEvent.Error.Error()
			r.StackTrace = e.LogEvent.StackTrace
		}
		r.LevelCC = levelColor(e.LogEvent.Level)
	} else {
		return
	}

	// A write error must not panic — a logger must not crash the service.
	_ = b.write(b.writer(), r)
}

func levelColor(level string) colorCode {
	switch level {
	case stack_backend.LevelDebug:
		return ccLvlDebug
	case stack_backend.LevelInfo:
		return ccLvlInfo
	case stack_backend.LevelWarn:
		return ccLvlWarn
	case stack_backend.LevelError:
		return ccLvlError
	default:
		return ccLvlDefault
	}
}

func (b Backend) Shutdown(ctx context.Context) {
}
