package bass

import (
	"errors"
	"fmt"
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
	return fmt.Sprintf("cannot decode %T into %T", err.Source, err.Destination)
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

// TODO: explain why
var ErrAbsolutePath = errors.New("absolute paths are not supported")

// TODO: better error
var ErrNoRuntime = errors.New("no runtime for platform")

var ErrInterrupted = errors.New("interrupted")

type TracedError struct {
	Err   error
	Trace *Trace
}

func (err TracedError) Unwrap() error {
	return err.Err
}

func (err TracedError) Error() string {
	msg := fmt.Sprintf("\x1b[31m%s\x1b[0m", err.Err)

	for _, frame := range err.Trace.Frames() {
		var form string
		if frame.Comment != "" {
			if strings.ContainsRune(frame.Comment, '\n') {
				for _, line := range strings.Split(frame.Comment, "\n") {
					form += fmt.Sprintf("; %s\n\t", line)
				}

				form += frame.Value.String()
			} else {
				form = fmt.Sprintf("%s ; %s", frame.Value, frame.Comment)
			}
		} else {
			form = frame.Value.String()
		}

		msg += fmt.Sprintf("\n\n%s\n\t%s", frame.Range, form)
	}

	return msg
}

type EncodeError struct {
	Value Value
}

func (err EncodeError) Error() string {
	return fmt.Sprintf("cannot encode %T: %s", err.Value, err.Value)
}
