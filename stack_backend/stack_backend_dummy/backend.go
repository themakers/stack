package stack_backend_dummy

import (
	"github.com/themakers/stack/stack_backend"
)

var _ stack_backend.Backend = Backend{}

type Backend struct{}

func New() stack_backend.Backend {
	return Backend{}
}

func (b Backend) Handle(_ stack_backend.Event) {
	//> Discard
}
