package runtimes

import (
	"context"
	"io"

	"github.com/vito/bass"
)

type Runtime interface {
	Run(context.Context, io.Writer, bass.Thunk) error
	Load(context.Context, bass.Thunk) (*bass.Scope, error)
	Export(context.Context, io.Writer, bass.Thunk) error
	ExportPath(context.Context, io.Writer, bass.ThunkPath) error
}

type poolKey struct{}

func WithPool(ctx context.Context, pool *Pool) context.Context {
	return context.WithValue(ctx, poolKey{}, pool)
}

func RuntimeFromContext(ctx context.Context, platform *bass.Platform) (Runtime, error) {
	pool := ctx.Value(poolKey{})
	if pool == nil {
		return nil, ErrNoRuntime
	}

	return pool.(*Pool).Select(platform)
}

var runtimes = map[string]InitFunc{}

// InitFunc is a Runtime constructor.
type InitFunc func(*Pool, *bass.Scope) (Runtime, error)

// Register installs a runtime under a given name.
//
// It should be called in a runtime's init() function with the runtime's
// constructor.
func RegisterRuntime(name string, init InitFunc) {
	runtimes[name] = init
}

// Init initializes the runtime registered under the given name.
func Init(name string, pool *Pool, config *bass.Scope) (Runtime, error) {
	init, found := runtimes[name]
	if !found {
		return nil, UnknownRuntimeError{
			Name: name,
		}
	}

	return init(pool, config)
}
