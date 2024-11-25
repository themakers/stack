package stack_backend_text

import (
	"bytes"
	"encoding/json"
	"github.com/fatih/color"
	"github.com/thearchitect/stack/stack_backend"
	"os"
	"time"
)

const timeFormat = "2006-01-02 15:04:05.000000000"

var _ stack_backend.Backend = Backend{}

type Backend struct{}

func New() stack_backend.Backend {
	return Backend{}
}

func (b Backend) Handle(e stack_backend.Event) {
	var (
		clrs logColors
		lev  string
		t    time.Time
		w    = bytes.NewBuffer([]byte{})
		dur  time.Duration
	)

	clrs.NestedAttrColor = color.New().AddRGB(128, 128, 128)
	clrs.OwnAttrColor = color.New(color.FgHiWhite)
	clrs.NameColor = color.New(color.FgMagenta)

	t = e.Time

	if e.Kind.Span {
		lev = stack_backend.LevelSpan
		clrs.LevelColor = color.New(color.FgWhite)
	} else if e.Kind.SpanEnd {
		lev = stack_backend.LevelSpanEnd
		clrs.LevelColor = color.New(color.FgHiWhite)
		t = e.EndTime
		dur = e.EndTime.Sub(e.Time)
	} else if e.Kind.Log {
		lev = e.Level
		switch e.Level {
		case stack_backend.LevelDebug:
			clrs.LevelColor = color.New(color.FgBlack, color.BgHiWhite)
		case stack_backend.LevelInfo:
			clrs.LevelColor = color.New(color.FgBlack, color.BgWhite)
		case stack_backend.LevelWarn:
			clrs.LevelColor = color.New(color.FgBlack, color.BgYellow)
		case stack_backend.LevelError:
			clrs.LevelColor = color.New(color.FgBlack, color.BgRed)
		default:
			clrs.LevelColor = color.New(color.FgBlue, color.BgWhite)
		}
	}

	w.WriteString(t.Format(timeFormat))
	w.WriteString(" ")
	w.WriteString(clrs.LevelColor.Sprintf(" %-5s ", lev))
	w.WriteString(" ")
	w.WriteString(clrs.NameColor.Sprint(e.Name))

	if dur != 0 {
		w.WriteString(" ")
		w.WriteString(clrs.OwnAttrColor.Sprint(dur))
	}

	//w.WriteString(" {")
	for i, f := range e.OwnAttrs {
		w.WriteString(" ")
		var v any
		switch f.Value.(type) {
		case stack_backend.RawAttrValue:
			v = f.Value
		default:
			v = jsonVal(f.Value)
		}
		w.WriteString(clrs.OwnAttrColor.Sprintf(`%s=%s`, f.Name, v))
		if i < len(e.OwnAttrs)-1 {
			w.WriteString(",")
		}
	}

	if len(e.OwnAttrs) > 0 && len(e.Attrs) > 0 {
		w.WriteString(",")
	}

	for i, f := range e.Attrs {
		w.WriteString(" ")
		var v any
		switch f.Value.(type) {
		case stack_backend.RawAttrValue:
			v = f.Value
		default:
			v = jsonVal(f.Value)
		}
		w.WriteString(clrs.NestedAttrColor.Sprintf(`%s=%s`, f.Name, v))
		if i < len(e.Attrs)-1 {
			w.WriteString(",")
		}
	}

	//w.WriteString(" }\n")
	w.WriteString("\n")

	if _, err := w.WriteTo(os.Stdout); err != nil {
		panic(err)
	}

}

type ColorOptions struct {
}

type Options struct {
	Colors ColorOptions
}

//var color = struct {
//}{}

type logColors struct {
	LevelColor      *color.Color
	NameColor       *color.Color
	NestedAttrColor *color.Color
	OwnAttrColor    *color.Color
}

func jsonVal(v any) string {
	if data, err := json.MarshalIndent(v, "", "  "); err != nil {
		panic(err)
	} else {
		return string(data)
	}
}
