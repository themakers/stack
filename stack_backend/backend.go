package stack_backend

// https://patorjk.com/software/taag/#p=display&f=RubiFont&t=test

//
// ▗▄▄▖  ▗▄▖  ▗▄▄▖▗▖ ▗▖▗▄▄▄▖▗▖  ▗▖▗▄▄▄
// ▐▌ ▐▌▐▌ ▐▌▐▌   ▐▌▗▞▘▐▌   ▐▛▚▖▐▌▐▌  █
// ▐▛▀▚▖▐▛▀▜▌▐▌   ▐▛▚▖ ▐▛▀▀▘▐▌ ▝▜▌▐▌  █
// ▐▙▄▞▘▐▌ ▐▌▝▚▄▄▖▐▌ ▐▌▐▙▄▄▖▐▌  ▐▌▐▙▄▄▀
//

type Backend interface {
	Handle(e Event)
}

//
// ▗▖ ▗▖▗▄▄▄▖▗▖   ▗▄▄▖ ▗▄▄▄▖▗▄▄▖  ▗▄▄▖
// ▐▌ ▐▌▐▌   ▐▌   ▐▌ ▐▌▐▌   ▐▌ ▐▌▐▌
// ▐▛▀▜▌▐▛▀▀▘▐▌   ▐▛▀▘ ▐▛▀▀▘▐▛▀▚▖ ▝▀▚▖
// ▐▌ ▐▌▐▙▄▄▖▐▙▄▄▖▐▌   ▐▙▄▄▖▐▌ ▐▌▗▄▄▞▘
//

// BackendFunc
type BackendFunc func(e Event)

var _ Backend = BackendFunc(func(e Event) {})

func (bFn BackendFunc) Handle(e Event) {
	bFn(e)
}

// MuxBackend
func MuxBackend(rules ...MuxBackendRule) Backend {
	return BackendFunc(func(e Event) {
		for _, rule := range rules {
			rule.TryHandle(e)
		}
	})
}

type MuxBackendRule struct {
	Backend Backend
	Kinds   Kind
	Filter  func(e Event) bool
}

func (r MuxBackendRule) TryHandle(e Event) {
	if r.Kinds != 0 && r.Kinds&e.Kind != 0 {
		r.Backend.Handle(e)
	} else if r.Filter != nil && r.Filter(e) {
		r.Backend.Handle(e)
	} else {
		//> SKIP
	}
}

// TeeBackend
func TeeBackend(backends ...Backend) Backend {
	var rules = make([]MuxBackendRule, len(backends))
	for i, backend := range backends {
		rules[i].Backend = backend
		rules[i].Kinds = 0xff
	}
	return MuxBackend(rules...)
}

// EventFilter
func EventFilter(backend Backend, filterFn func(e *Event) bool) Backend {
	return BackendFunc(func(e Event) {
		if filterFn(&e) {
			backend.Handle(e)
		}
	})
}
