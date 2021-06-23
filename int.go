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
