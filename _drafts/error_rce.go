package _drafts

import (
	"errors"
	"github.com/themakers/stack"

	"github.com/DataDog/gostackparse"
)

var (
	_ error     = (*rootCauseError)(nil)
	_ unwrapOne = (*rootCauseError)(nil)
	_ errorIs   = (*rootCauseError)(nil)
)

type rootCauseError struct {
	id       string
	spanID   string
	cause    error
	trace    *gostackparse.Goroutine
	attrs    []stack.Attr
	ownAttrs []stack.Attr
}

func (e *rootCauseError) Error() string {
	return e.cause.Error()
}

func (e *rootCauseError) Unwrap() error {
	return e.cause
}

func (e *rootCauseError) Is(err error) bool {
	return errors.Is(e.cause, err)
}

func findRootCauseError(err error) (*rootCauseError, bool) {
	if e, ok := err.(*rootCauseError); ok {
		return e, true
	} else if e, ok := err.(unwrapOne); ok {
		return findRootCauseError(e.Unwrap())
	} else if e, ok := err.(unwrapMany); ok {
		for _, e := range e.Unwrap() {
			if e, ok := findRootCauseError(e); ok {
				return e, ok
			}
		}
		return nil, false
	} else {
		return nil, false
	}
}
