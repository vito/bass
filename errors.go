package bass

import "fmt"

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
