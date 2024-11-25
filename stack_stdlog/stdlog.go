package stack_stdlog

import (
	"context"
	"log"
	"log/slog"
)

func Hijack(ctx context.Context) {
	slog.SetDefault(slog.New(NewSlogHandler(ctx)))
	log.SetOutput(NewLogWriter(ctx))
}
