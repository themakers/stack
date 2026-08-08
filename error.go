package stack

import "github.com/themakers/stack/stack_backend"

type errorWithStackTrace interface {
	error
	StackTrace() stack_backend.StackTrace
}

type tracedError struct {
	cause error
	trace stack_backend.StackTrace
}

func (e *tracedError) Error() string {
	return e.cause.Error()
}

func (e *tracedError) Unwrap() error {
	return e.cause
}

func (e *tracedError) StackTrace() stack_backend.StackTrace {
	return e.trace
}
