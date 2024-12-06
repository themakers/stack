package stack_backend

import "context"

// https://patorjk.com/software/taag/#p=display&f=RubiFont&t=test

//
// ▗▄▄▖  ▗▄▖  ▗▄▄▖▗▖ ▗▖▗▄▄▄▖▗▖  ▗▖▗▄▄▄
// ▐▌ ▐▌▐▌ ▐▌▐▌   ▐▌▗▞▘▐▌   ▐▛▚▖▐▌▐▌  █
// ▐▛▀▚▖▐▛▀▜▌▐▌   ▐▛▚▖ ▐▛▀▀▘▐▌ ▝▜▌▐▌  █
// ▐▙▄▞▘▐▌ ▐▌▝▚▄▄▖▐▌ ▐▌▐▙▄▄▖▐▌  ▐▌▐▙▄▄▀
//

type Backend interface {
	Handle(e Event)
	Shutdown(ctx context.Context)
}

func Shutdown(ctx context.Context) {
	Get(ctx).Backend.Shutdown(context.TODO())
}

//
// ▗▖ ▗▖▗▄▄▄▖▗▖   ▗▄▄▖ ▗▄▄▄▖▗▄▄▖  ▗▄▄▖
// ▐▌ ▐▌▐▌   ▐▌   ▐▌ ▐▌▐▌   ▐▌ ▐▌▐▌
// ▐▛▀▜▌▐▛▀▀▘▐▌   ▐▛▀▘ ▐▛▀▀▘▐▛▀▚▖ ▝▀▚▖
// ▐▌ ▐▌▐▙▄▄▖▐▙▄▄▖▐▌   ▐▙▄▄▖▐▌ ▐▌▗▄▄▞▘
//

// backendFuncs
type BackendFuncs struct {
	HandleFn   func(e Event)
	ShutdownFn func(ctx context.Context)
}

var _ Backend = BackendFuncs{}

func (bf BackendFuncs) Handle(e Event) {
	if bf.HandleFn != nil {
		bf.HandleFn(e)
	}
}
func (bf BackendFuncs) Shutdown(ctx context.Context) {
	if bf.ShutdownFn != nil {
		bf.ShutdownFn(ctx)
	}
}

// MuxBackend
func MuxBackend(rules ...MuxBackendRule) Backend {
	return BackendFuncs{
		HandleFn: func(e Event) {
			for _, rule := range rules {
				rule.TryHandle(e)
			}
		},
		ShutdownFn: func(ctx context.Context) {

		},
	}
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
	return BackendFuncs{
		HandleFn: func(e Event) {
			if filterFn(&e) {
				backend.Handle(e)
			}
		},
	}
}

// TODO
func LevelFilter(backend Backend) Backend {
	return EventFilter(backend, func(e *Event) bool {
		return true
	})
}
