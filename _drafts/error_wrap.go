package _drafts

import (
	"errors"
	"fmt"
	"github.com/thearchitect/stack"
)

var (
	_ error            = (*wrappedError)(nil)
	_ stack.unwrapMany = (*wrappedError)(nil)
	_ stack.errorIs    = (*wrappedError)(nil)
)

type wrappedError struct {
	err   error
	cause error
}

func Wrap(cause, err error) error {
	if err == nil && cause != nil {
		return cause
	} else if err != nil && cause == nil {
		return err
	} else if err == nil && cause == nil {
		return nil
	}

	return &wrappedError{
		err:   err,
		cause: cause,
	}
}

func (e *wrappedError) Error() string {
	return fmt.Sprintf("%v (cause: %v)", e.err, e.cause)
}

func (e *wrappedError) Unwrap() []error {
	return []error{e.err, e.cause}
}

func (e *wrappedError) Is(err error) bool {
	if errors.Is(e.cause, err) {
		return true
	} else if errors.Is(e.err, err) {
		return true
	} else {
		return false
	}
}
