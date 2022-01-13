package runtimes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vito/bass"
)

// NoRuntimeError is returned when a platform has no runtime associated to it.
type NoRuntimeError struct {
	Platform bass.Platform
}

func (err NoRuntimeError) Error() string {
	return fmt.Sprintf("no runtime configured for %s", err.Platform)
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
