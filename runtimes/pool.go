package runtimes

import (
	"fmt"

	"github.com/vito/bass"
)

// Pool is the full set of platform <-> runtime pairs configured by the user.
type Pool struct {
	Bass     bass.Runtime
	Runtimes []Assoc
}

// Assoc associates a platform to a runtime.
type Assoc struct {
	Platform bass.Platform
	Runtime  bass.Runtime
}

// Pool is a 'union' runtime which delegates each call to the appropriate
// runtime based on the Thunk's platform.
// var _ bass.Runtime = &Pool{}

// NewPool initializes all runtimes in the given configuration.
func NewPool(config *bass.Config) (*Pool, error) {
	pool := &Pool{}
	pool.Bass = NewBass(pool)

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

// Select chooses a runtime appropriate for the requested platform.
func (pool *Pool) Select(platform *bass.Platform) (bass.Runtime, error) {
	if platform == nil {
		return pool.Bass, nil
	}

	for _, runtime := range pool.Runtimes {
		if platform.CanSelect(runtime.Platform) {
			return runtime.Runtime, nil
		}
	}

	return nil, NoRuntimeError{*platform}
}

// All returns all available runtimes.
func (pool *Pool) All() []bass.Runtime {
	var all []bass.Runtime
	for _, assoc := range pool.Runtimes {
		all = append(all, assoc.Runtime)
	}

	return all
}
