package _drafts

import (
	"bytes"
	"context"
	"github.com/DataDog/gostackparse"
	"github.com/themakers/stack"
	"github.com/themakers/stack/stack_backend"
	"runtime/debug"
)

type unwrapOne interface { // from errors.is
	Unwrap() error
}

type unwrapMany interface { // from errors.is
	Unwrap() []error
}

type errorIs interface { // from errors.is
	Is(error) bool
}

func NewError(ctx context.Context, err error, attrs ...stack.A) error {
	if err == nil {
		return nil
	}

	s := stack_backend.Get(ctx)

	//> The first registered error becomes 'root cause'
	if _, found := findRootCauseError(err); !found {
		err = &rootCauseError{
			id:       stack_backend.GenerateID(),
			spanID:   s.ID,
			cause:    err,
			trace:    stacktrace(),
			attrs:    s.Attrs,
			ownAttrs: attrs,
		}
	} else {

	}

	return err
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
