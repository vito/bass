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

func (value Annotated) Eval(env *Env) (Value, error) {
	res, err := value.Value.Eval(env)
	if err != nil {
		return nil, AnnotatedError{
			Value: value.Value,
			Range: value.Range,
			Err:   err,
		}
	}

	if value.Comment != "" {
		env.Commentary = append(env.Commentary, Annotated{
			Comment: value.Comment,
			Value:   res,
		})

		var sym Symbol
		if err := res.Decode(&sym); err == nil {
			env.Docs[sym] = value.Comment
		}
	}

	return res, nil
}
