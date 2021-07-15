package bass

import "fmt"

type Applicative interface {
	Unwrap() Combiner
}

type Wrapped struct {
	Underlying Combiner
}

var _ Applicative = Wrapped{}

func (app Wrapped) Unwrap() Combiner {
	return app.Underlying
}

var _ Value = Wrapped{}

func (value Wrapped) Equal(other Value) bool {
	var o Wrapped
	return other.Decode(&o) == nil && value == o
}

func (value Wrapped) String() string {
	var op *Operative
	if err := value.Underlying.Decode(&op); err == nil {
		if op.Eformal == (Ignore{}) {
			return NewList(
				Symbol("fn"),
				op.Formals,
				op.Body,
			).String()
		}
	}

	return NewList(
		Symbol("wrap"),
		value.Underlying,
	).String()
}

func (value Wrapped) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Wrapped:
		*x = value
		return nil
	case *Applicative:
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

// Eval returns the value.
func (value Wrapped) Eval(env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Combiner = Wrapped{}

// Call evaluates the value in the envionment and calls the underlying
// combiner with the result.
func (combiner Wrapped) Call(val Value, env *Env, cont Cont) ReadyCont {
	var list List
	err := val.Decode(&list)
	if err != nil {
		return cont.Call(nil, fmt.Errorf("call applicative: %w", err))
	}

	return ToCons(list).Eval(env, Chain(cont, func(res Value) Value {
		return combiner.Underlying.Call(res, env, cont)
	}))
}
