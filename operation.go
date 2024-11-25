package stack

import (
	"bytes"
	"runtime"
	"runtime/debug"

	"github.com/DataDog/gostackparse"
)

func operation(skip int) (funcName string, file string, line int) {
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

func stacktrace() *gostackparse.Goroutine {
	const skip = 2

	s := debug.Stack()

	goroutines, errs := gostackparse.Parse(bytes.NewReader(s))

	if len(errs) > 0 {
		println("malformed stacktrace:", string(s))
	}

	goroutine := goroutines[0]

	goroutine.Stack = goroutine.Stack[skip:]

	return goroutine
}
