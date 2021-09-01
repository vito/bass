package bass

import "context"

type Symbol string

var _ Value = Symbol("")

func (value Symbol) String() string {
	return string(value)
}

func (value Symbol) Keyword() Keyword {
	return Keyword(value)
}

func (value Symbol) Equal(other Value) bool {
	var o Symbol
	return other.Decode(&o) == nil && value == o
}

func (value Symbol) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Symbol:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Bindable:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

// Eval returns the value.
func (value Symbol) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	res, found := scope.Get(value.Keyword())
	if !found {
		return cont.Call(nil, UnboundError{value})
	}

	return cont.Call(res, nil)
}

var _ Bindable = Symbol("")

func (binding Symbol) Bind(scope *Scope, val Value) error {
	scope.Set(binding.Keyword(), val)
	return nil
}
