package stack_stdlog

import (
	"context"
	"io"
	"strings"

	"github.com/themakers/stack"
)

// NewLogWriter returns an io.Writer that forwards standard log package lines
// to stack.Info on top of ctx. The writer used to silently discard input.
func NewLogWriter(ctx context.Context) io.Writer {
	return writerFunc(func(p []byte) (n int, err error) {
		msg := strings.TrimRight(string(p), "\n")
		if msg != "" {
			stack.Info(ctx, msg)
		}
		return len(p), nil
	})
}

type writerFunc func(p []byte) (n int, err error)

func (f writerFunc) Write(p []byte) (n int, err error) {
	return f(p)
}
