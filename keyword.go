package bass

import (
	"context"
	"fmt"
)

type Keyword int

var _ Value = Keyword(0)

func NewKeyword(str string) Keyword {
	return Keyword(NewSymbol(str))
}

func (value Keyword) String() string {
	return fmt.Sprintf(":%s", symbols[value])
}

func (value Keyword) Symbol() Symbol {
	return Symbol(value)
}

func (value Keyword) Equal(other Value) bool {
	var o Keyword
	return other.Decode(&o) == nil && value == o
}

func (value Keyword) Decode(dest interface{}) error {
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

// Eval converts the Keyword to a Symbol.
func (value Keyword) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value.Symbol(), nil)
}

var _ Bindable = Keyword(0)

func (binding Keyword) Bind(_ *Scope, val Value) error {
	return BindConst(binding, val)
}
