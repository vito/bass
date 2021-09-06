package bass

import "context"

type Null struct{}

func (Null) String() string {
	return "null"
}

func (Null) Equal(other Value) bool {
	var o Null
	return other.Decode(&o) == nil
}

// Decode decodes into a Null or into bool (setting false).
func (value Null) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Null:
		return nil
	case *Bindable:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil

	case *bool:
		// null is equivalent to false in (if) and (not)
		//
		// this feels a little surprising given Decode is typically used for
		// asserting satisfiability of interfaces, not converting values. but it's
		// also a pragmatic way to ensure invariants like this are enforced
		// consistently language-wide.
		//
		// XXX: keep an eye on this; reexamine tradeoffs if it becomes problematic
		*x = false
		return nil

	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

// Eval returns the value.
func (value Null) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

// MarshalJSON marshals as `null`.
func (Null) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

var _ Bindable = Null{}

func (binding Null) Bind(_ *Scope, val Value) error {
	return BindConst(binding, val)
}
