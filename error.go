package stack

import (
	"context"
	"github.com/thearchitect/stack/span"
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

	s := span.Get(ctx)

	//> The first registered error becomes 'root cause'
	if _, found := findRootCauseError(err); !found {
		err = &rootCauseError{
			id:       span.GenerateID(),
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
