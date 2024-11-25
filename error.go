package stack

import (
	"context"
	"github.com/thearchitect/stack/stack_backend"
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

func NewError(ctx context.Context, err error, attrs ...Attr) error {
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
