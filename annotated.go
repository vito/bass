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

func (value Annotated) Eval(env *Env, cont Cont) ReadyCont {
	next := cont
	if value.Comment != "" {
		next = Continue(func(res Value) Value {
			env.Commentary = append(env.Commentary, Annotated{
				Comment: value.Comment,
				Value:   res,
			})

			var sym Symbol
			if err := res.Decode(&sym); err == nil {
				env.Docs[sym] = value.Comment
			}

			return cont.Call(res, nil)
		})
	}

	return value.Value.Eval(env, &Traced{
		Cont:  next,
		Form:  value.Value,
		Range: value.Range,
	})
}

type Traced struct {
	Cont

	Form  Value
	Range Range
}

func (traced *Traced) Call(res Value, err error) ReadyCont {
	if err != nil {
		return traced.Cont.Call(nil, AnnotatedError{
			Value: traced.Form,
			Range: traced.Range,
			Err:   err,
		})
	}

	return traced.Cont.Call(res, err)
}
