package bass

import "fmt"

type Combiner interface {
	Value

	Call(Value, *Env) (Value, error)
}

type Applicative struct {
	Underlying Combiner
}

var _ Combiner = Applicative{}

func (value Applicative) Decode(dest interface{}) error {
	return fmt.Errorf("TODO: Applicative.Decode")
}

// Eval returns the value.
func (value Applicative) Eval(env *Env) (Value, error) {
	return value, nil
}

// Call evaluates the value in the envionment and calls the underlying
// combiner with the result.
func (combiner Applicative) Call(val Value, env *Env) (Value, error) {
	res, err := val.Eval(env)
	if err != nil {
		return nil, err
	}

	return combiner.Underlying.Call(res, env)
}
