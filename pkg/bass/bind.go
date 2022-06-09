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

func (value Bind) Decode(dest any) error {
	switch x := dest.(type) {
	case *Bindable:
		*x = value
		return nil
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

var _ Bindable = Bind{}

func (bind Bind) Bind(ctx context.Context, bindScope *Scope, cont Cont, val Value, _ ...Annotated) ReadyCont {
	var valScope *Scope
	if err := val.Decode(&valScope); err != nil {
		return cont.Call(nil, BindMismatchError{
			Need: bind,
			Have: val,
		})
	}

	if len(bind)%2 != 0 {
		// TODO: better error
		return cont.Call(nil, ErrBadSyntax)
	}

	if len(bind) == 0 {
		return cont.Call(bind, nil)
	}

	vf, vb, rest := bind[0], bind[1], bind[2:]

	var valBinding Bindable
	if err := vb.Decode(&valBinding); err != nil {
		return cont.Call(nil, CannotBindError{vb})
	}

	var kw Keyword
	var kwWithDefault List
	var valDefault Value
	if err := vf.Decode(&kwWithDefault); err == nil {
		vals, err := ToSlice(kwWithDefault)
		if err != nil {
			return cont.Call(nil, err)
		}

		if len(vals) != 2 {
			return cont.Call(nil, ErrBadSyntax)
		}

		if err := vals[0].Decode(&kw); err != nil {
			// TODO: better error
			return cont.Call(nil, err)
		}

		valDefault = vals[1]
	} else if err := vf.Decode(&kw); err != nil {
		return cont.Call(nil, err)
	}

	sym := kw.Symbol()

	doBind := Continue(func(subVal Value) Value {
		return valBinding.Bind(ctx, bindScope, Continue(func(Value) Value {
			return rest.Bind(ctx, bindScope, cont, valScope)
		}), subVal)
	})

	subVal, found := valScope.Get(sym)
	if found {
		return doBind.Call(subVal, nil)
	} else if valDefault != nil {
		return valDefault.Eval(ctx, bindScope, doBind)
	} else {
		return cont.Call(nil, UnboundError{
			Symbol: sym,
			Scope:  valScope,
		})
	}
}

func (bind Bind) EachBinding(cb func(Symbol, Range) error) error {
	if len(bind)%2 != 0 {
		// TODO: better error
		return ErrBadSyntax
	}

	for i, vb := range bind {
		if i%2 != 1 {
			continue
		}

		var valBinding Bindable
		if err := vb.Decode(&valBinding); err != nil {
			return CannotBindError{vb}
		}

		if err := valBinding.EachBinding(cb); err != nil {
			return fmt.Errorf("in %s: %w", bind, err)
		}
	}

	return nil
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
