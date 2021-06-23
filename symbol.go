package bass

type Symbol string

func (value Symbol) Decode(dest interface{}) error {
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
