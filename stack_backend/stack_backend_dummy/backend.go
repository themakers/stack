package stack_backend_dummy

import (
	"github.com/thearchitect/stack/stack_backend"
)

var _ stack_backend.Backend = Backend{}

type Backend struct{}

func New() stack_backend.Backend {
	return Backend{}
}

func (b Backend) HandleNewRecord(r stack_backend.Event) {
}

func (b Backend) HandleSpanEnd(r stack_backend.Event) {
}
