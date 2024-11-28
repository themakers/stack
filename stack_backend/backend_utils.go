package stack_backend

var _ Backend = BackendFunc(func(e Event) {})

type BackendFunc func(e Event)

func (bFn BackendFunc) Handle(e Event) {
	bFn(e)
}

func TeeBackend(backends ...Backend) Backend {
	return nil
}

type MuxBackendRule struct {
	Backend Backend
	Kinds   Kind
}

func MuxBackend(rules ...MuxBackendRule) Backend {
	return nil
}

func EventFilter(backend Backend, filterFn func(e *Event) bool) Backend {
	return BackendFunc(func(e Event) {
		if filterFn(&e) {
			backend.Handle(e)
		}
	})
}
