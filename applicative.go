package bass

import "fmt"

type Applicative struct {
	Underlying Combiner
}

var _ Value = Applicative{}

func (value Applicative) Equal(other Value) bool {
	var o Applicative
	return other.Decode(&o) == nil && value == o
}

func (value Applicative) String() string {
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

func (value Applicative) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Combiner:
		*x = value
		return nil
	case *Applicative:
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
func (value Applicative) Eval(env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Combiner = Applicative{}

// Call evaluates the value in the envionment and calls the underlying
// combiner with the result.
func (combiner Applicative) Call(val Value, env *Env, cont Cont) ReadyCont {
	var list List
	err := val.Decode(&list)
	if err != nil {
		return cont.Call(nil, fmt.Errorf("call applicative: %w", err))
	}

	return ToCons(list).Eval(env, Continue(func(res Value) Value {
		return combiner.Underlying.Call(res, env, cont)
	}))
}
