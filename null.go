package bass

type Null struct{}

func (Null) String() string {
	return "null"
}

// Decode decodes into a Null or into bool (setting false).
func (value Null) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Null:
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
func (value Null) Eval(*Env) (Value, error) {
	return value, nil
}
