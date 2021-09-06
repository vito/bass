package runtimes

import (
	"fmt"

	"github.com/vito/bass"
)

// NoRuntimeError is returned when a platform has no runtime associated to it.
type NoRuntimeError struct {
	Platform *bass.Scope
}

func (err NoRuntimeError) Error() string {
	return fmt.Sprintf("no runtime configured for %s", err.Platform)
}
