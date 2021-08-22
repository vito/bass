package bass

import (
	"context"
	"io"
)

type Runtime interface {
	Run(context.Context, io.Writer, Workload) error
	Load(context.Context, Workload) (*Env, error)
	Export(context.Context, io.Writer, Workload, FilesystemPath) error
}

type runtimeKey struct{}

func WithRuntime(ctx context.Context, runtime Runtime) context.Context {
	return context.WithValue(ctx, runtimeKey{}, runtime)
}

func RuntimeFromContext(ctx context.Context) (Runtime, error) {
	runtime := ctx.Value(runtimeKey{})
	if runtime == nil {
		return nil, ErrNoRuntime
	}

	return runtime.(Runtime), nil
}

var runtimes = map[string]InitFunc{}

// InitFunc is a Runtime constructor.
type InitFunc func(Runtime, Object) (Runtime, error)

// Register installs a runtime under a given name.
//
// It should be called in a runtime's init() function with the runtime's
// constructor.
func RegisterRuntime(name string, init InitFunc) {
	runtimes[name] = init
}

// InitRuntie initializes the runtime registered under the given name.
func InitRuntime(name string, external Runtime, config Object) (Runtime, error) {
	init, found := runtimes[name]
	if !found {
		return nil, UnknownRuntimeError{
			Name: name,
		}
	}

	return init(external, config)
}
