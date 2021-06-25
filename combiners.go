package bass

type Combiner interface {
	Value

	Call(Value, *Env) (Value, error)
}
