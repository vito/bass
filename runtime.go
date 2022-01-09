package bass

import (
	"context"
	"errors"
	"io"
)

type RuntimePool interface {
	Select(platform *Platform) (Runtime, error)
}

type Runtime interface {
	Resolve(context.Context, ThunkImageRef) (ThunkImageRef, error)
	Run(context.Context, io.Writer, Thunk) error
	Load(context.Context, Thunk) (*Scope, error)
	Export(context.Context, io.Writer, Thunk) error
	ExportPath(context.Context, io.Writer, ThunkPath) error
}

type poolKey struct{}

func WithRuntimePool(ctx context.Context, pool RuntimePool) context.Context {
	return context.WithValue(ctx, poolKey{}, pool)
}

func RuntimePoolFromContext(ctx context.Context, platform *Platform) (Runtime, error) {
	pool := ctx.Value(poolKey{})
	if pool == nil {
		return nil, ErrNoRuntimePool
	}

	return pool.(RuntimePool).Select(platform)
}

// ErrNoRuntimePool is returned when the context.Context does not have a
// runtime pool set.
var ErrNoRuntimePool = errors.New("runtime not initialized")
