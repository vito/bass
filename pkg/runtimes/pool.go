package runtimes

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/vito/bass/pkg/bass"
)

// Pool is the full set of platform <-> runtime pairs configured by the user.
type Pool struct {
	Runtimes []Assoc
}

// Assoc associates a platform to a runtime.
type Assoc struct {
	Platform bass.Platform
	Runtime  bass.Runtime
}

// NewPool initializes all runtimes in the given configuration.
func NewPool(ctx context.Context, config *bass.Config) (*Pool, error) {
	pool := &Pool{}

	for _, config := range config.Runtimes {
		runtime, err := Init(ctx, config.Runtime, pool, config.Config)
		if err != nil {
			return nil, fmt.Errorf("init %s runtime for platform %s: %w", config.Runtime, config.Platform, err)
		}

		pool.Runtimes = append(pool.Runtimes, Assoc{
			Platform: config.Platform,
			Runtime:  runtime,
		})
	}

	return pool, nil
}

// Select chooses a runtime appropriate for the requested platform.
func (pool *Pool) Select(platform bass.Platform) (bass.Runtime, error) {
	for _, runtime := range pool.Runtimes {
		if platform.CanSelect(runtime.Platform) {
			return runtime.Runtime, nil
		}
	}

	return nil, NoRuntimeError{
		Platform:    platform,
		AllRuntimes: pool.Runtimes,
	}
}

// All returns all available runtimes.
func (pool *Pool) All() ([]bass.Runtime, error) {
	var all []bass.Runtime
	for _, assoc := range pool.Runtimes {
		all = append(all, assoc.Runtime)
	}

	return all, nil
}

// Close closes each runtime.
func (pool *Pool) Close() error {
	var errs error
	for _, assoc := range pool.Runtimes {
		if err := assoc.Runtime.Close(); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}
