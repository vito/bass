package bass

import "context"

type Null struct{}

func (Null) String() string {
	return "null"
}

func (value Null) Equal(other Value) bool {
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
func (value Null) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

// MarshalJSON marshals as `null`.
func (value Null) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

var _ Bindable = Null{}

func (binding Null) Bind(env *Env, val Value) error {
	return BindConst(binding, val)
}
