package runtimes

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/morikuni/aec"
	"github.com/vito/bass/pkg/bass"
)

// NoRuntimeError is returned when a platform has no runtime associated to it.
type NoRuntimeError struct {
	Platform bass.Platform

	AllRuntimes []Assoc
}

func (err NoRuntimeError) Error() string {
	return fmt.Sprintf("no runtime available for platform: %s", err.Platform)
}

func (err NoRuntimeError) NiceError(w io.Writer, outer error) error {
	fmt.Fprintln(w, aec.RedF.Apply(outer.Error()))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "configured runtimes: %d", len(err.AllRuntimes))
	if len(err.AllRuntimes) > 0 {
		fmt.Fprintln(w)
		for _, assoc := range err.AllRuntimes {
			fmt.Fprintf(w, "* %s", assoc.Platform)
		}
	}
	return nil
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
