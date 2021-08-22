package bass

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type CannotBindError struct {
	Have Value
}

func (err CannotBindError) Error() string {
	return fmt.Sprintf("bind: cannot bind to %s", err.Have)
}

type BindMismatchError struct {
	Need Value
	Have Value
}

func (err BindMismatchError) Error() string {
	// TODO: better error
	return fmt.Sprintf("bind: need %s, have %s", err.Need, err.Have)
}

type DecodeError struct {
	Source      interface{}
	Destination interface{}
}

func (err DecodeError) Error() string {
	return fmt.Sprintf("cannot decode %s (%T) into %T", err.Source, err.Source, err.Destination)
}

type UnboundError struct {
	Symbol Symbol
}

func (err UnboundError) Error() string {
	return fmt.Sprintf("unbound symbol: %s", err.Symbol)
}

type ArityError struct {
	Name     string
	Need     int
	Variadic bool
	Have     int
}

func (err ArityError) Error() string {
	var msg string
	if err.Variadic {
		msg = "%s arity: need at least %d arguments, given %d"
	} else {
		msg = "%s arity: need %d arguments, given %d"
	}

	return fmt.Sprintf(
		msg,
		err.Name,
		err.Need,
		err.Have,
	)
}

var ErrBadSyntax = errors.New("bad syntax")

type BadKeyError struct {
	Value
}

func (err BadKeyError) Error() string {
	return fmt.Sprintf("objects must have :keyword keys; have %s", err.Value)
}

var ErrEndOfSource = errors.New("end of source")

var ErrInterrupted = errors.New("interrupted")

type EncodeError struct {
	Value Value
}

func (err EncodeError) Error() string {
	return fmt.Sprintf("cannot encode %T: %s", err.Value, err.Value)
}

type ExtendError struct {
	Parent Path
	Child  Path
}

func (err ExtendError) Error() string {
	return fmt.Sprintf(
		"cannot extend path %s (%T) with %s (%T)",
		err.Parent,
		err.Parent,
		err.Child,
		err.Child,
	)
}

// ErrNoRuntime is returned when the context.Context does not have a
// runtime set.
//
// This really should never happen, but erroring is better than
// panicking.
var ErrNoRuntime = errors.New("runtime not initialized")

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
