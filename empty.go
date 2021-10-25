package bass

import (
	"context"
)

type Empty struct{}

func (Empty) MarshalJSON() ([]byte, error) {
	return []byte("[]"), nil
}

func (*Empty) UnmarshalJSON(payload []byte) error {
	var x []interface{}
	err := UnmarshalJSON(payload, &x)
	if err != nil {
		return err
	}

	return nil
}

func (Empty) Equal(other Value) bool {
	var o Empty
	return other.Decode(&o) == nil
}

func (Empty) String() string {
	return "()"
}

func (value Empty) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Empty:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *List:
		*x = value
		return nil
	case *Bindable:
		*x = value
		return nil
	}

	return decodeSlice(value, dest)
}

// Eval returns the value.
func (value Empty) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func (Empty) First() Value {
	return Empty{}
}

func (Empty) Rest() Value {
	return Empty{}
}

var _ Bindable = Empty{}

func (binding Empty) Bind(_ context.Context, _ *Scope, cont Cont, val Value, _ ...Annotated) ReadyCont {
	return cont.Call(binding, BindConst(binding, val))
}

func (Empty) EachBinding(func(Symbol, Range) error) error {
	return nil
}
