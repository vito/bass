package bass

type Operative struct {
	Formals Value
	Eformal Value
	Body    Value

	Env *Env
}

var _ Value = (*Operative)(nil)

var _ Combiner = (*Operative)(nil)

func (value *Operative) String() string {
	return NewList(
		Symbol("op"),
		value.Formals,
		value.Eformal,
		value.Body,
	).String()
}

func (value *Operative) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Combiner:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value *Operative) Eval(*Env) (Value, error) {
	// TODO: test
	return value, nil
}

func (value *Operative) Call(val Value, env *Env) (Value, error) {
	sub := NewEnv(value.Env)

	err := sub.Define(value.Formals, val)
	if err != nil {
		return nil, err
	}

	err = sub.Define(value.Eformal, env)
	if err != nil {
		return nil, err
	}

	return value.Body.Eval(sub)
}
