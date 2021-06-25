package bass

import "fmt"

type Pair struct {
	A Value
	D Value
}

var _ Value = Pair{}

func (value Pair) String() string {
	out := "("

	var list List = value
	for list != (Empty{}) {
		out += list.First().String()

		switch rest := list.Rest().(type) {
		case Empty:
			list = Empty{}
		case List:
			out += " "
			list = rest
		default:
			out += " . "
			out += rest.String()
			list = Empty{}
		}
	}

	out += ")"

	return out
}

func (value Pair) Decode(dest interface{}) error {
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

var _ List = Pair{}

func (value Pair) First() Value {
	return value.A
}

func (value Pair) Rest() Value {
	return value.D
}

// Pair combines the first operand with the second operand.
//
// If the first value is not a Combiner, an error is returned.
func (value Pair) Eval(env *Env) (Value, error) {
	f, err := value.A.Eval(env)
	if err != nil {
		return nil, err
	}

	combiner, ok := f.(Combiner)
	if !ok {
		return nil, fmt.Errorf("cannot use %T as a combiner - TODO: better error", f)
	}

	return combiner.Call(value.D, env)
}
