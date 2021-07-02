package bass

type Empty struct{}

func (value Empty) Equal(other Value) bool {
	var o Empty
	return other.Decode(&o) == nil
}

func (value Empty) String() string {
	return "()"
}

func (value Empty) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Empty:
		*x = value
		return nil
	case *List:
		*x = value
		return nil
	}

	return DecodeError{
		Source:      value,
		Destination: dest,
	}
}

// Eval returns the value.
func (value Empty) Eval(env *Env, cont Cont) (ReadyCont, error) {
	return cont.Call(value), nil
}

func (Empty) First() Value {
	return Empty{}
}

func (Empty) Rest() Value {
	return Empty{}
}
