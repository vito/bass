package bass

// Apply is a Pair which Calls its first value against the remaining values.
//
// If the first value is not Callable, an error is returned.
type Apply Pair

var _ Value = Apply{}

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
