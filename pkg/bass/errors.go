package bass

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/pflag"
	"github.com/spy16/slurp/reader"
)

// NiceError is an error that is able to provide some extra guidance to the
// user.
//
// Strive for all errors returned by Bass to become a NiceError.
type NiceError interface {
	error

	NiceError(io.Writer) error
}

type FlagError struct {
	Err   error
	Flags *pflag.FlagSet
}

func (err FlagError) Error() string {
	return err.Err.Error()
}

func (err FlagError) NiceError(w io.Writer) error {
	fmt.Fprintf(w, "\x1b[31m%s\x1b[0m\n", err)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "flags:")

	// this is a little hokey, but it should be fine
	cp := *err.Flags
	cp.SetOutput(w)
	cp.PrintDefaults()

	return nil
}

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
	Source      any
	Destination any
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

// ReadError is returned when the reader trips on a syntax token.
type ReadError struct {
	Err reader.Error
}

func (err ReadError) Error() string {
	return fmt.Sprintf("%s: %s", err.Err.Begin, err.Err)
}
