package bass

type Ignore struct{}

var _ Value = Ignore{}

func (value Ignore) String() string {
	return "_"
}

func (value Ignore) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Ignore:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value Ignore) Eval(*Env) (Value, error) {
	return value, nil
}
