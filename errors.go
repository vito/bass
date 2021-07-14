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

type AnnotatedError struct {
	Err error

	Form    Value
	Range   Range
	Comment string
}

func (err AnnotatedError) Unwrap() error {
	return err.Err
}

func (err AnnotatedError) Error() string {
	var form string
	if err.Comment != "" {
		if strings.ContainsRune(err.Comment, '\n') {
			for _, line := range strings.Split(err.Comment, "\n") {
				form += fmt.Sprintf("; %s\n\t", line)
			}

			form += err.Form.String()
		} else {
			form = fmt.Sprintf("%s ; %s", err.Form, err.Comment)
		}
	} else {
		form = err.Form.String()
	}

	return fmt.Sprintf("\x1b[31m%s\x1b[0m\n\n%s\n\t%s", err.Err, err.Range, form)
}

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
