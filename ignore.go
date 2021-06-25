package bass

type Ignore struct{}

var _ Value = Ignore{}

func (value Ignore) String() string {
	return "_"
}

func (value Ignore) Decode(interface{}) error {
	return nil
}

func (value Ignore) Eval(*Env) (Value, error) {
	return value, nil
}
