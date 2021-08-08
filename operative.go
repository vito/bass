package bass

import "context"

type Operative struct {
	Formals Bindable
	Eformal Bindable
	Body    Value

	Env *Env
}

var _ Value = (*Operative)(nil)

func (value *Operative) Equal(other Value) bool {
	var o *Operative
	return other.Decode(&o) == nil && value == o
}

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
	case **Operative:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value *Operative) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

func (value *Operative) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
	// TODO: test
	return cont.Call(value, nil)
}

var _ Combiner = (*Operative)(nil)

func (combiner *Operative) Call(ctx context.Context, val Value, env *Env, cont Cont) ReadyCont {
	sub := NewEnv(combiner.Env)

	err := combiner.Formals.Bind(sub, val)
	if err != nil {
		return cont.Call(nil, err)
	}

	err = combiner.Eformal.Bind(sub, env)
	if err != nil {
		return cont.Call(nil, err)
	}

	return combiner.Body.Eval(ctx, sub, cont)
}
