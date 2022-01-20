package bass

import (
	"context"
	"errors"
	"io"
	"time"
)

type RuntimePool interface {
	Select(*Platform) (Runtime, error)
	All() []Runtime
}

type Runtime interface {
	Resolve(context.Context, ThunkImageRef) (ThunkImageRef, error)
	Run(context.Context, io.Writer, Thunk) error
	Load(context.Context, Thunk) (*Scope, error)
	Export(context.Context, io.Writer, Thunk) error
	ExportPath(context.Context, io.Writer, ThunkPath) error
	Prune(context.Context, PruneOpts) error
}

// PruneOpts contains parameters to fine-tune the pruning behavior. These
// parameters are best-effort; not all runtimes are expected to support every
// option.
type PruneOpts struct {
	// Prune everything.
	All bool

	// Keep data last used within the duration.
	KeepDuration time.Duration

	// Keep
	KeepBytes int64
}

type poolKey struct{}

func WithRuntimePool(ctx context.Context, pool RuntimePool) context.Context {
	return context.WithValue(ctx, poolKey{}, pool)
}

func RuntimePoolFromContext(ctx context.Context) (RuntimePool, error) {
	pool := ctx.Value(poolKey{})
	if pool == nil {
		return nil, ErrNoRuntimePool
	}

	return pool.(RuntimePool), nil
}

func RuntimeFromContext(ctx context.Context, platform *Platform) (Runtime, error) {
	pool := ctx.Value(poolKey{})
	if pool == nil {
		return nil, ErrNoRuntimePool
	}

	return pool.(RuntimePool).Select(platform)
}

// ErrNoRuntimePool is returned when the context.Context does not have a
// runtime pool set.
var ErrNoRuntimePool = errors.New("runtime not initialized")
