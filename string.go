package bass

type String string

func (value String) Decode(dest interface{}) error {
	switch x := dest.(type) {
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
func (value String) Eval(*Env) (Value, error) {
	return value, nil
}
