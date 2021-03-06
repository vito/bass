package bass

import "context"

type Operative struct {
	Bindings     Bindable
	ScopeBinding Bindable
	Body         Value
	StaticScope  *Scope
}

var _ Value = (*Operative)(nil)

func (value *Operative) Equal(other Value) bool {
	var o *Operative
	return other.Decode(&o) == nil && value == o
}

func (value *Operative) String() string {
	return NewList(
		Symbol("op"),
		value.Bindings,
		value.ScopeBinding,
		value.Body,
	).String()
}

func (value *Operative) Decode(dest any) error {
	switch x := dest.(type) {
	case **Operative:
		*x = value
		return nil
	case *Combiner:
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

func (value *Operative) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

func (value *Operative) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Combiner = (*Operative)(nil)

func (combiner *Operative) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	sub := NewEmptyScope(combiner.StaticScope)

	return combiner.Bindings.Bind(ctx, sub, Continue(func(Value) Value {
		return combiner.ScopeBinding.Bind(ctx, sub, Continue(func(Value) Value {
			return combiner.Body.Eval(ctx, sub, cont)
		}), scope)
	}), val)
}
