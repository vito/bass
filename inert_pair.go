package bass

type InertPair Pair

func NewInertList(vals ...Value) List {
	var list List = Empty{}
	for i := len(vals) - 1; i >= 0; i-- {
		list = InertPair{
			A: vals[i],
			D: list,
		}
	}

	return list
}

func Inert(list List) List {
	switch x := list.(type) {
	case Empty:
		return x
	case Pair:
		switch d := x.D.(type) {
		case List:
			return InertPair{
				A: x.First(),
				D: Inert(d),
			}
		default:
			return InertPair{
				A: x.First(),
				D: d,
			}
		}
	default:
		return x
	}
}

func (value InertPair) String() string {
	out := "["

	var list List = value
	for list != (Empty{}) {
		out += list.First().String()

		var ok bool
		list, ok = list.Rest().(List)
		if !ok {
			out += " ."
		}

		if list != (Empty{}) {
			out += " "
		}
	}
	out += "]"

	return out
}

func (value InertPair) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *List:
		*x = value
		return nil
	}

	return DecodeError{
		Source:      value,
		Destination: dest,
	}
}

// Eval evaluates both values in the pair.
func (value InertPair) Eval(env *Env) (Value, error) {
	a, err := value.A.Eval(env)
	if err != nil {
		return nil, err
	}

	d, err := value.D.Eval(env)
	if err != nil {
		return nil, err
	}

	return Pair{
		A: a,
		D: d,
	}, nil
}

func (value InertPair) First() Value {
	return value.A
}

func (value InertPair) Rest() Value {
	return value.D
}
