package bass

import (
	"context"
	"fmt"
)

// Keyword is a value that evaluates to a symbol.
type Keyword string

var _ Value = Keyword("")

// String returns the keyword's symbol in keyword notation.
func (value Keyword) String() string {
	return fmt.Sprintf(":%s", string(value))
}

// Symbol converts the keyword to a symbol.
func (value Keyword) Symbol() Symbol {
	return Symbol(value)
}

// Equal returns true if the other value is a Keyword representing the same
// symbol.
func (value Keyword) Equal(other Value) bool {
	var o Keyword
	return other.Decode(&o) == nil && value == o
}

// Decode coerces the keyword into another compatible type.
func (value Keyword) Decode(dest any) error {
	switch x := dest.(type) {
	case *Keyword:
		*x = value
		return nil
	case *Bindable:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

// Eval returns the keyword's symbol.
func (value Keyword) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value.Symbol(), nil)
}

var _ Bindable = Keyword("")

// Bind performs constant value binding; it only succeeds against an equal
// value and introduces no bindings.
func (binding Keyword) Bind(_ context.Context, _ *Scope, cont Cont, val Value, _ ...Annotated) ReadyCont {
	return cont.Call(binding, BindConst(binding.Symbol(), val))
}

// EachBinding does nothing.
func (Keyword) EachBinding(func(Symbol, Range) error) error {
	return nil
}
