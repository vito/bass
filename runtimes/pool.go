package runtimes

import (
	"context"
	"fmt"
	"io"

	"github.com/vito/bass"
)

// Pool is the full set of platform <-> runtime pairs configured by the user.
type Pool struct {
	Bass     bass.Runtime
	Runtimes []Assoc
}

// Assoc associates a platform to a runtime.
type Assoc struct {
	Platform *bass.Scope
	Runtime  bass.Runtime
}

// Pool is a 'union' runtime which delegates each call to the appropriate
// runtime based on the Workload's platform.
var _ bass.Runtime = &Pool{}

// NewPool initializes all runtimes in the given configuration.
func NewPool(config *bass.Config) (*Pool, error) {
	pool := &Pool{}
	pool.Bass = NewBass(pool)

	for _, config := range config.Runtimes {
		runtime, err := bass.InitRuntime(config.Runtime, pool, config.Config)
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
func (pool Pool) Run(ctx context.Context, w io.Writer, workload bass.Workload) error {
	if workload.Platform == nil {
		return pool.Bass.Run(ctx, w, workload)
	}

	for _, runtime := range pool.Runtimes {
		if workload.Platform.Equal(runtime.Platform) {
			return runtime.Runtime.Run(ctx, w, workload)
		}
	}

	return NoRuntimeError{workload.Platform}
}

// Load delegates to the runtime matching the workload's platform, or returns
// NoRuntimeError if none match.
func (pool Pool) Load(ctx context.Context, workload bass.Workload) (*bass.Scope, error) {
	if workload.Platform == nil {
		return pool.Bass.Load(ctx, workload)
	}

	for _, runtime := range pool.Runtimes {
		if workload.Platform.Equal(runtime.Platform) {
			return runtime.Runtime.Load(ctx, workload)
		}
	}

	return nil, NoRuntimeError{workload.Platform}
}

// Export delegates to the runtime matching the workload's platform, or returns
// NoRuntimeError if none match.
func (pool Pool) Export(ctx context.Context, w io.Writer, workload bass.Workload, path bass.FilesystemPath) error {
	if workload.Platform == nil {
		return pool.Bass.Export(ctx, w, workload, path)
	}

	for _, runtime := range pool.Runtimes {
		if workload.Platform.Equal(runtime.Platform) {
			return runtime.Runtime.Export(ctx, w, workload, path)
		}
	}

	return NoRuntimeError{workload.Platform}
}
