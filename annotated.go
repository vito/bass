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

func (inner Range) IsWithin(outer Range) bool {
	if inner.Start.Ln < outer.Start.Ln {
		return false
	}

	if inner.End.Ln > outer.End.Ln {
		return false
	}

	if inner.Start.Ln == outer.Start.Ln {
		if inner.Start.Col < outer.Start.Col {
			return false
		}
	}

	if inner.End.Ln == outer.End.Ln {
		if inner.End.Col > outer.End.Col {
			return false
		}
	}

	return true
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
	case *Bindable:
		var inner Bindable
		if err := value.Value.Decode(&inner); err != nil {
			return err
		}

		*x = AnnotatedBinding{
			Bindable: inner,
			Range:    value.Range,
		}

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
				scope.SetDoc(sym, comment)
			}

			return cont.Call(res, nil)
		})
	}

	return value.Value.Eval(ctx, scope, WithFrame(ctx, &value, next))
}

type AnnotatedBinding struct {
	Bindable
	Range Range
}

var _ Bindable = AnnotatedBinding{}

func (binding AnnotatedBinding) Bind(scope *Scope, value Value, _ ...Annotated) error {
	return binding.Bindable.Bind(scope, value)
}

func (binding AnnotatedBinding) EachBinding(cb func(Symbol, Range) error) error {
	return binding.Bindable.EachBinding(func(s Symbol, r Range) error {
		if r == (Range{}) {
			return cb(s, binding.Range)
		} else {
			return cb(s, r)
		}
	})
}
