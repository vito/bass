package bass

import "fmt"

type Applicative struct {
	Underlying Combiner
}

var _ Combiner = Applicative{}

func (value Applicative) String() string {
	switch x := value.Underlying.(type) {
	case *Operative:
		if x.Eformal == (Ignore{}) {
			return NewList(
				Symbol("fn"),
				x.Formals,
				x.Body,
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
func (value Applicative) Eval(env *Env) (Value, error) {
	return value, nil
}

// Call evaluates the value in the envionment and calls the underlying
// combiner with the result.
func (combiner Applicative) Call(val Value, env *Env) (Value, error) {
	list, ok := val.(List)
	if !ok {
		return nil, fmt.Errorf("TODO: operand must be a List, got %T", val)
	}

	res, err := Inert(list).Eval(env)
	if err != nil {
		return nil, err
	}

	return combiner.Underlying.Call(res, env)
}
