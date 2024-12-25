package stack_backend_text

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/DataDog/gostackparse"
	"github.com/fatih/color"
	"github.com/themakers/stack/stack_backend"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const timeFormat = "2006-01-02 15:04:05.000000000"

var _ stack_backend.Backend = Backend{}

type Backend struct{}

func New() stack_backend.Backend {
	return Backend{}
}

//	type logColors struct {
//		LevelColor      *color.Color
//		NameColor       *color.Color
//		NestedAttrColor *color.Color
//		OwnAttrColor    *color.Color
//	}
type record struct {
	Time      time.Time
	TimeColor *color.Color

	Level      string
	LevelColor *color.Color

	File          string
	Line          int
	FileLineColor *color.Color

	Name      string
	NameColor *color.Color

	Error      string
	ErrorColor *color.Color

	Duration      time.Duration
	DurationColor *color.Color

	OwnAttrs      []stack_backend.Attr
	OwnAttrsColor *color.Color

	NestedAttrs      []stack_backend.Attr
	NestedAttrsColor *color.Color

	StackTrace *gostackparse.Goroutine
}

func (b Backend) write(w io.Writer, isTTY bool, r record) error {
	buf := bytes.NewBuffer([]byte{})

	buf.WriteString(r.TimeColor.Sprint(r.Time.Format(timeFormat)))
	buf.WriteString(" ")
	buf.WriteString(r.LevelColor.Sprintf(" %-5s ", r.Level))
	if len(r.File) > 0 {
		buf.WriteString(" ")
		buf.WriteString(r.FileLineColor.Sprintf("%s:%d", filepath.Base(r.File), r.Line))
	}
	buf.WriteString(" ")
	buf.WriteString(r.NameColor.Sprint(r.Name))

	if r.Duration != 0 {
		buf.WriteString(" ")
		buf.WriteString(r.OwnAttrsColor.Sprint(r.Duration))
	}

	if r.Error != "" {
		buf.WriteString(" ")
		buf.WriteString(r.ErrorColor.Sprint(r.Error))
	}

	//buf.WriteString(" {")
	for i, f := range r.OwnAttrs {
		buf.WriteString(" ")
		var v any
		switch f.Value.(type) {
		case stack_backend.RawAttrValue:
			v = f.Value
		default:
			v = jsonVal(f.Value)
		}
		buf.WriteString(r.OwnAttrsColor.Sprintf(`%s=%s`, f.Name, v))
		if i < len(r.OwnAttrs)-1 {
			buf.WriteString(",")
		}
	}

	if len(r.OwnAttrs) > 0 && len(r.NestedAttrs) > 0 {
		buf.WriteString(r.NestedAttrsColor.Sprint(","))
	}

	for i, f := range r.NestedAttrs {
		buf.WriteString(" ")
		var v any
		switch f.Value.(type) {
		case stack_backend.RawAttrValue:
			v = f.Value
		default:
			v = jsonVal(f.Value)
		}
		buf.WriteString(r.NestedAttrsColor.Sprintf(`%s=%s`, f.Name, v))
		if i < len(r.NestedAttrs)-1 {
			buf.WriteString(",")
		}
	}

	//buf.WriteString(" }\n")
	buf.WriteString("\n")

	if r.StackTrace != nil {
		for _, frm := range r.StackTrace.Stack {
			buf.WriteString("     ••• ")
			buf.WriteString(frm.Func)
			buf.WriteString("\n")
			buf.WriteString("                 ")

			seg := strings.Split(frm.File, string(rune(filepath.Separator)))
			if len(seg) >= 5 {
				seg = seg[len(seg)-5:]
			}

			buf.WriteString(filepath.Join(seg...))
			buf.WriteString(":")
			buf.WriteString(strconv.Itoa(frm.Line))
			buf.WriteString(" ••• ")
			buf.WriteString("\n")
		}
	}

	if _, err := buf.WriteTo(w); err != nil {
		return err
	} else {
		return nil
	}
}

func (b Backend) Handle(e stack_backend.Event) {
	var r record

	r.TimeColor = color.New()
	r.FileLineColor = color.New()
	r.NestedAttrsColor = color.New().AddRGB(128, 128, 128)
	r.OwnAttrsColor = color.New(color.FgHiWhite)
	r.NameColor = color.New(color.FgMagenta)
	r.ErrorColor = color.New(color.FgRed, color.BgWhite)

	if e.Kind&stack_backend.KindSpan != 0 {
		r.Name = e.State.Span.Name
		r.Level = stack_backend.LevelSpan
		r.LevelColor = color.New(color.FgWhite)
		r.File = e.State.Span.File
		r.Line = e.State.Span.Line
		r.Time = e.State.Span.Time
		r.OwnAttrs = e.State.Span.Attrs
	} else if e.Kind&stack_backend.KindSpanEnd != 0 {
		r.Name = e.State.Span.Name
		r.Level = stack_backend.LevelSpanEnd
		r.LevelColor = color.New(color.FgHiWhite)
		r.Time = e.State.Span.EndTime
		r.Duration = e.State.Span.EndTime.Sub(e.State.Span.Time)
		r.OwnAttrs = e.State.Span.Attrs
		if e.State.Span.Error != nil {
			r.Error = fmt.Sprint(e.State.Span.Error)
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
			r.Error = fmt.Sprint(e.LogEvent.Error)
			r.StackTrace = e.LogEvent.StackTrace
		}
		switch e.LogEvent.Level {
		case stack_backend.LevelDebug:
			r.LevelColor = color.New(color.FgBlack, color.BgHiWhite)
		case stack_backend.LevelInfo:
			r.LevelColor = color.New(color.FgBlack, color.BgWhite)
		case stack_backend.LevelWarn:
			r.LevelColor = color.New(color.FgBlack, color.BgYellow)
		case stack_backend.LevelError:
			r.LevelColor = color.New(color.FgBlack, color.BgRed)
		default:
			r.LevelColor = color.New(color.FgBlue, color.BgWhite)
		}
	}

	if err := b.write(os.Stdout, true, r); err != nil {
		panic(err)
	}
}

func jsonVal(v any) string {
	if data, err := json.MarshalIndent(v, "", "  "); err != nil {
		panic(err)
	} else {
		return string(data)
	}
}

func (b Backend) Shutdown(ctx context.Context) {
}
