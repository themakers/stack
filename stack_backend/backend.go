package stack_backend

type Backend interface {
	Handle(e Event)
}
