package bass

import (
	"context"
	"fmt"
)

type String string

func (value String) String() string {
	// TODO: account for differences in escape sequences
	return fmt.Sprintf("%q", string(value))
}

func (value String) Equal(other Value) bool {
	var o String
	return other.Decode(&o) == nil && value == o
}

func (value String) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *String:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Bindable:
		*x = value
		return nil
	case *string:
		*x = string(value)
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

// Eval returns the value.
func (value String) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Bindable = String("")

func (binding String) Bind(_ *Scope, val Value, _ ...Annotated) error {
	return BindConst(binding, val)
}

func (String) EachBinding(func(Symbol, Range) error) error {
	return nil
}
