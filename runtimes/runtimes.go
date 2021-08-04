package runtimes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vito/bass"
)

var runtimes = map[string]InitFunc{}

// InitFunc is a Runtime constructor.
type InitFunc func(*Pool, bass.Object) (Runtime, error)

// Register installs a runtime under a given name.
//
// It should be called in a runtime's init() function with the runtime's
// constructor.
func Register(name string, init InitFunc) {
	runtimes[name] = init
}

// UnknownRuntimeError is returned when an unknown runtime is configured.
type UnknownRuntimeError struct {
	Name string
}

func (err UnknownRuntimeError) Error() string {
	available := []string{}
	for name := range runtimes {
		available = append(available, name)
	}

	sort.Strings(available)

	return fmt.Sprintf(
		"unknown runtime: %s; available: %s",
		err.Name,
		strings.Join(available, ", "),
	)
}

// Init initializes the named runtime.
func Init(name string, pool *Pool, config bass.Object) (Runtime, error) {
	init, found := runtimes[name]
	if !found {
		return nil, UnknownRuntimeError{
			Name: name,
		}
	}

	return init(pool, config)
}
