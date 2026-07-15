package stack_backend_json

import (
	"context"
	"encoding/json"
	"os"

	"github.com/themakers/stack/stack_backend"
)

var _ stack_backend.Backend = Backend{}

type Backend struct{}

func New() stack_backend.Backend {
	return Backend{}
}

func (b Backend) Handle(e stack_backend.Event) {
	// The logger must not panic: a marshal error becomes a placeholder, a
	// stdout write error is ignored (EPIPE must not crash the service).
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		data = []byte(`{"error":"stack_backend_json: marshal failed"}`)
	}
	_, _ = os.Stdout.Write(data)
}

func (b Backend) Shutdown(ctx context.Context) {
}
