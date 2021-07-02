package bass

import (
	"fmt"

	"github.com/spy16/slurp/reader"
)

type Annotated struct {
	Value

	Range Range

	Comment string
}

type Range struct {
	Start, End reader.Position
}

func (r Range) String() string {
	return fmt.Sprintf("%s\t%d:%d..%d:%d", r.Start.File, r.Start.Ln, r.Start.Col, r.End.Ln, r.End.Col)
}

func (value Annotated) Eval(env *Env, cont Cont) (ReadyCont, error) {
	next := cont
	if value.Comment != "" {
		next = Continue(func(res Value) (Value, error) {
			env.Commentary = append(env.Commentary, Annotated{
				Comment: value.Comment,
				Value:   res,
			})

			var sym Symbol
			if err := res.Decode(&sym); err == nil {
				env.Docs[sym] = value.Comment
			}

			return cont.Call(res), nil
		})
	}

	rdy, err := value.Value.Eval(env, next)
	if err != nil {
		return nil, AnnotatedError{
			Value: value.Value,
			Range: value.Range,
			Err:   err,
		}
	}

	return rdy, nil
}
