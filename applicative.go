package bass

import (
	"context"
)

type Applicative interface {
	Combiner

	Unwrap() Combiner
}

type Wrapped struct {
	Underlying Combiner
}

func Wrap(comb Combiner) Applicative {
	return Wrapped{comb}
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
		if op.ScopeFormal == (Ignore{}) {
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

func (value Wrapped) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

// Eval returns the value.
func (value Wrapped) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Combiner = Wrapped{}

// Call evaluates the value in the scope and calls the underlying
// combiner with the result.
func (combiner Wrapped) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	arg := val

	var pair Pair
	if err := val.Decode(&pair); err == nil {
		arg = ToCons(pair)
	}

	return arg.Eval(ctx, scope, Continue(func(res Value) Value {
		return combiner.Underlying.Call(ctx, res, scope, cont)
	}))
}
