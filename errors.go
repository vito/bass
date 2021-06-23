package bass

import "fmt"

type DecodeError struct {
	Source      interface{}
	Destination interface{}
}

func (err DecodeError) Error() string {
	return fmt.Sprintf("cannot decode %T into %T", err.Source, err.Destination)
}
