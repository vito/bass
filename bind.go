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
	newScope := NewEmptyScope(scope)

	return NewConsList(value...).Eval(ctx, newScope, Continue(func(vals Value) Value {
		content, err := ToSlice(vals.(List))
		if err != nil {
			return cont.Call(nil, fmt.Errorf("to slice: %w", err))
		}

		newScope.Parents = nil

		return scopeBuilder(content).Build(ctx, newScope, cont)
	}))
}

type scopeBuilder []Value

func (vs scopeBuilder) Build(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	if len(vs) == 0 {
		return cont.Call(scope, nil)
	}

	v := vs[0]

	var sym Symbol
	if err := v.Decode(&sym); err == nil {
		if len(vs) < 2 {
			// TODO: better error
			return cont.Call(nil, ErrBadSyntax)
		}

		val := vs[1]

		var ann Annotated
		if err := v.Decode(&ann); err == nil {
			val = Annotated{
				Value: val,
				Meta:  ann.Meta,
			}
		}

		return sym.Bind(ctx, scope, Continue(func(Value) Value {
			return vs[2:].Build(ctx, scope, cont)
		}), val)
	}

	var parent *Scope
	if err := v.Decode(&parent); err == nil {
		scope.Parents = append(scope.Parents, parent)

		return vs[1:].Build(ctx, scope, cont)
	}

	// un-named value?

	// TODO: better error
	return cont.Call(nil, fmt.Errorf("bind: %w: %s", ErrBadSyntax, vs))
}
