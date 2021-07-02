package bass

type Symbol string

func (value Symbol) String() string {
	return string(value)
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
func (value Symbol) Eval(env *Env, cont Cont) ReadyCont {
	res, found := env.Get(value)
	if !found {
		return cont.Call(nil, UnboundError{value})
	}

	return cont.Call(res, nil)
}
