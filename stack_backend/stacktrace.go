package stack_backend

import (
	"encoding/json"
	"runtime"
	"strconv"
)

// StackTrace is a lazily-resolved call stack: raw program counters captured
// at the error site. Capturing costs a single small allocation (8·depth
// bytes); symbol resolution (function names, files, lines) is deferred until
// a backend actually renders the trace via Frames/String/MarshalJSON.
//
// All stack trace logic lives here in the core; backends only consume this
// API and must not resolve PCs themselves.
type StackTrace []uintptr

// Frame is one resolved stack trace entry.
type Frame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// Stacktrace captures the current call stack as raw PCs. The PC buffer lives
// on the stack ([64]uintptr) — runtime.Callers does not retain it; only the
// resulting slice goes to the heap (the single allocation of this function).
func Stacktrace(skip int) StackTrace {
	const baseSkip = 3 // runtime.Callers + Stacktrace + the calling log()/Error()

	var pcbuf [64]uintptr
	n := runtime.Callers(baseSkip+skip, pcbuf[:])
	if n == 0 {
		return nil
	}

	st := make(StackTrace, n)
	copy(st, pcbuf[:n])
	return st
}

// Frames resolves the PCs and yields the frames in order, allowing renderers
// to write directly into their output buffer without materializing an
// intermediate slice. Frame strings share memory with pclntab (no copies).
func (st StackTrace) Frames(yield func(Frame) bool) {
	if len(st) == 0 {
		return
	}
	frames := runtime.CallersFrames(st)
	for {
		frame, more := frames.Next()
		if frame.PC != 0 || frame.Function != "" || frame.File != "" {
			if !yield(Frame{Function: frame.Function, File: frame.File, Line: frame.Line}) {
				return
			}
		}
		if !more {
			return
		}
	}
}

// String renders the resolved trace as multi-line text in the conventional
// "function\n\tfile:line" form (suitable for e.g. OTel exception.stacktrace).
func (st StackTrace) String() string {
	if len(st) == 0 {
		return ""
	}
	buf := make([]byte, 0, len(st)*64)
	st.Frames(func(f Frame) bool {
		buf = append(buf, f.Function...)
		buf = append(buf, "\n\t"...)
		buf = append(buf, f.File...)
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(f.Line), 10)
		buf = append(buf, '\n')
		return true
	})
	return string(buf)
}

// MarshalJSON renders the trace as an array of resolved frames — the raw PCs
// are meaningless outside the producing process (e.g. for the json backend).
func (st StackTrace) MarshalJSON() ([]byte, error) {
	if len(st) == 0 {
		return []byte("null"), nil
	}
	frames := make([]Frame, 0, len(st))
	st.Frames(func(f Frame) bool {
		frames = append(frames, f)
		return true
	})
	return json.Marshal(frames)
}
