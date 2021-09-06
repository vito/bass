package bass

import "context"

type Combiner interface {
	Value

	Call(context.Context, Value, *Scope, Cont) ReadyCont
}
