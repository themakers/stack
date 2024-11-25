package stack_backend_json

import (
	"encoding/json"
	"os"

	"github.com/thearchitect/stack/stack_backend"
)

var _ stack_backend.Backend = Backend{}

type Backend struct{}

func New() stack_backend.Backend {
	return Backend{}
}

func (b Backend) Handle(e stack_backend.Event) {
	if data, err := json.MarshalIndent(e, "", "  "); err != nil {
		panic(err)
	} else if _, err := os.Stdout.Write(data); err != nil {
		panic(err)
	}
}
