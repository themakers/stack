package stack_stdlog

import (
	"context"
	"io"
)

func NewLogWriter(ctx context.Context) io.Writer {
	// TODO
	return writerFunc(func(p []byte) (n int, err error) {

		return 0, err
	})
}

type writerFunc func(p []byte) (n int, err error)

func (f writerFunc) Write(p []byte) (n int, err error) {
	return f(p)
}
