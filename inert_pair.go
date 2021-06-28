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
	var empty Empty
	if err := list.Decode(&empty); err == nil {
		return list
	}

	var rest List
	if err := list.Rest().Decode(&rest); err == nil {
		return InertPair{
			A: list.First(),
			D: Inert(rest),
		}
	}

	return InertPair{
		A: list.First(),
		D: list.Rest(),
	}
}

func (value InertPair) String() string {
	return formatList(value, "[", "]")
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
