package bass

import "strconv"

type Int int

func (value Int) String() string {
	return strconv.Itoa(int(value))
}

func (value Int) Equal(other Value) bool {
	var o Int
	return other.Decode(&o) == nil && value == o
}

func (value Int) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Int:
		*x = value
		return nil
	case *int:
		*x = int(value)
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

// Eval returns the value.
func (value Int) Eval(env *Env, cont Cont) (ReadyCont, error) {
	return cont.Call(value), nil
}
