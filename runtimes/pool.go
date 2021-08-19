package runtimes

import (
	"context"
	"fmt"
	"io"

	"github.com/vito/bass"
)

// Pool is the full set of platform <-> runtime pairs configured by the user.
type Pool struct {
	Runtimes []Assoc
}

// Assoc associates a platform to a runtime.
type Assoc struct {
	Platform bass.Object
	Runtime  Runtime
}

var _ Runtime = &Pool{}

// NewPool initializes all runtimes in the given configuration.
func NewPool(config *bass.Config) (*Pool, error) {
	pool := &Pool{}

	// include bass runtime by default
	pool.Runtimes = append(pool.Runtimes, Assoc{
		Platform: bass.Object{},
		Runtime:  NewBass(pool),
	})

	for _, config := range config.Runtimes {
		runtime, err := Init(config.Runtime, pool, config.Config)
		if err != nil {
			return nil, fmt.Errorf("init runtime for platform %s: %w", config.Platform, err)
		}

		pool.Runtimes = append(pool.Runtimes, Assoc{
			Platform: config.Platform,
			Runtime:  runtime,
		})
	}

	return pool, nil
}

// Run delegates to the runtime matching the workload's platform, or returns
// NoRuntimeError if none match.
func (pool Pool) Run(ctx context.Context, workload bass.Workload) error {
	for _, runtime := range pool.Runtimes {
		if workload.Platform.Equal(runtime.Platform) {
			return runtime.Runtime.Run(ctx, workload)
		}
	}

	return NoRuntimeError{workload.Platform}
}

// Response delegates to the runtime matching the workload's platform, or
// returns NoRuntimeError if none match.
func (pool Pool) Response(ctx context.Context, w io.Writer, workload bass.Workload) error {
	for _, runtime := range pool.Runtimes {
		if workload.Platform.Equal(runtime.Platform) {
			return runtime.Runtime.Response(ctx, w, workload)
		}
	}

	return NoRuntimeError{workload.Platform}
}

// Env delegates to the runtime matching the workload's platform, or
// returns NoRuntimeError if none match.
func (pool Pool) Env(ctx context.Context, workload bass.Workload) (*bass.Env, error) {
	for _, runtime := range pool.Runtimes {
		if workload.Platform.Equal(runtime.Platform) {
			return runtime.Runtime.Env(ctx, workload)
		}
	}

	return nil, NoRuntimeError{workload.Platform}
}

// Export delegates to the runtime matching the workload's platform, or returns
// NoRuntimeError if none match.
func (pool Pool) Export(ctx context.Context, w io.Writer, workload bass.Workload, path bass.FilesystemPath) error {
	for _, runtime := range pool.Runtimes {
		if workload.Platform.Equal(runtime.Platform) {
			return runtime.Runtime.Export(ctx, w, workload, path)
		}
	}

	return NoRuntimeError{workload.Platform}
}
