package bass

import "fmt"

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
func (value String) Eval(env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}
