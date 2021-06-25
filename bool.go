package bass

type Bool bool

func (value Bool) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *bool:
		*x = bool(value)
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

// Eval returns the value.
func (value Bool) Eval(*Env) (Value, error) {
	return value, nil
}
