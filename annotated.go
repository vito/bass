package bass

import (
	"context"
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
	return fmt.Sprintf("%s:%d:%d..%d:%d", r.Start.File, r.Start.Ln, r.Start.Col, r.End.Ln, r.End.Col)
}

func (value Annotated) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Annotated:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	default:
		return value.Value.Decode(dest)
	}
}

func (value Annotated) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

func (value Annotated) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	next := cont
	if value.Comment != "" {
		next = Continue(func(res Value) Value {
			comment := Annotated{
				Comment: value.Comment,
				Range:   value.Range,
				Value:   res,
			}

			scope.Commentary = append(scope.Commentary, comment)

			var sym Symbol
			if err := res.Decode(&sym); err == nil {
				scope.Docs[sym] = comment
			}

			return cont.Call(res, nil)
		})
	}

	return value.Value.Eval(ctx, scope, WithFrame(ctx, &value, next))
}
