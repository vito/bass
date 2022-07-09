package runtimes

import (
	"context"

	"github.com/vito/bass/pkg/bass"
)

var runtimes = map[string]InitFunc{}

// InitFunc is a Runtime constructor.
type InitFunc func(context.Context, bass.RuntimePool, *bass.Scope) (bass.Runtime, error)

// Register installs a runtime under a given name.
//
// It should be called in a runtime's init() function with the runtime's
// constructor.
func RegisterRuntime(name string, init InitFunc) {
	runtimes[name] = init
}

// Init initializes the runtime registered under the given name.
func Init(ctx context.Context, name string, pool bass.RuntimePool, config *bass.Scope) (bass.Runtime, error) {
	init, found := runtimes[name]
	if !found {
		return nil, UnknownRuntimeError{
			Name: name,
		}
	}

	return init(ctx, pool, config)
}
