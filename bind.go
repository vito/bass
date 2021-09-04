package bass

import (
	"context"
	"fmt"
)

type Bind []Value

var _ Value = Bind(nil)

func (value Bind) String() string {
	return formatList(NewList(value...), "{", "}")
}

func (value Bind) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Bind:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value Bind) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

func (value Bind) Equal(ovalue Value) bool {
	var other Bind
	if err := ovalue.Decode(&other); err != nil {
		return false
	}

	if len(value) != len(other) {
		return false
	}

	for i := range value {
		if !value[i].Equal(other[i]) {
			return false
		}
	}

	return true
}

func (value Bind) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	doc := NewEmptyScope(scope)

	return NewConsList(value...).Eval(ctx, doc, Continue(func(vals Value) Value {
		init, err := ToSlice(vals.(List))
		if err != nil {
			return cont.Call(nil, fmt.Errorf("to slice: %w", err))
		}

		bound := NewEmptyScope()
		bound.Commentary = doc.Commentary
		bound.Docs = doc.Docs

		var binding Bindable
		for i, val := range init {
			if binding != nil {
				err := binding.Bind(bound, val)
				if err != nil {
					return cont.Call(nil, err)
				}

				binding = nil
				continue
			}

			var sym Symbol
			if err := val.Decode(&sym); err == nil {
				binding = sym
				continue
			}

			var parent *Scope
			if err := val.Decode(&parent); err != nil {
				// TODO: better error
				return cont.Call(nil, fmt.Errorf("value %d: %w", i+1, ErrBadSyntax))
			}

			bound.Parents = append(bound.Parents, parent)
		}

		return cont.Call(bound, nil)
	}))
}
