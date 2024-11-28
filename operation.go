package stack

import (
	"runtime"
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
