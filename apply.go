package bass

import "fmt"

// Apply is a Pair which Calls its first value against the remaining values.
//
// If the first value is not Callable, an error is returned.
type Apply Pair

var _ Value = Apply{}

func (value Apply) String() string {
	return Pair(value).String()
}

func (value Apply) Decode(val interface{}) error {
	panic("TODO: Apply.Decode")
	return Pair(value).Decode(val)
}

var _ List = Apply{}

func (value Apply) First() Value {
	return value.A
}

func (value Apply) Rest() Value {
	return value.D
}

func (value Apply) Eval(env *Env) (Value, error) {
	f, err := value.A.Eval(env)
	if err != nil {
		return nil, err
	}

	combiner, ok := f.(Combiner)
	if !ok {
		return nil, fmt.Errorf("cannot use %T as a combiner - TODO: better error", combiner)
	}

	return combiner.Call(value.D, env)
}
