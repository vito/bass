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
