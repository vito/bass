package bass

import (
	"context"
)

type Empty struct{}

func (value Empty) MarshalJSON() ([]byte, error) {
	return []byte("[]"), nil
}

func (value *Empty) UnmarshalJSON(payload []byte) error {
	var x []interface{}
	err := UnmarshalJSON(payload, &x)
	if err != nil {
		return err
	}

	return nil
}

func (value Empty) Equal(other Value) bool {
	var o Empty
	return other.Decode(&o) == nil
}

func (value Empty) String() string {
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
func (value Empty) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func (Empty) First() Value {
	return Empty{}
}

func (Empty) Rest() Value {
	return Empty{}
}

var _ Bindable = Empty{}

func (binding Empty) Bind(env *Env, val Value) error {
	if val.Decode(&binding) != nil {
		return BindMismatchError{
			Need: binding,
			Have: val,
		}
	}

	return nil
}
