package bass

type Combiner interface {
	Value

	Call(Value, *Env, Cont) ReadyCont
}
