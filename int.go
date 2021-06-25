package bass

type Int int

func (value Int) Decode(dest interface{}) error {
	switch x := dest.(type) {
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
func (value Int) Eval(*Env) (Value, error) {
	return value, nil
}
