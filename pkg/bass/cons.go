package bass

import "context"

type Cons Pair

func NewConsList(vals ...Value) List {
	var list List = Empty{}
	for i := len(vals) - 1; i >= 0; i-- {
		list = Cons{
			A: vals[i],
			D: list,
		}
	}

	return list
}

func ToCons(list List) List {
	var empty Empty
	if err := list.Decode(&empty); err == nil {
		return list
	}

	var rest List
	if err := list.Rest().Decode(&rest); err == nil {
		return Cons{
			A: list.First(),
			D: ToCons(rest),
		}
	}

	return Cons{
		A: list.First(),
		D: list.Rest(),
	}
}

func (value Cons) String() string {
	return formatList(value, "[", "]")
}

func (value Cons) Equal(other Value) bool {
	var o Cons
	if err := other.Decode(&o); err != nil {
		return false
	}

	return value.A.Equal(o.A) && value.D.Equal(o.D)
}

func (value Cons) Decode(dest any) error {
	switch x := dest.(type) {
	case *Cons:
		*x = value
		return nil
	case *List:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Bindable:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value Cons) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

// Eval evaluates both values in the pair.
func (value Cons) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return value.A.Eval(ctx, scope, Continue(func(a Value) Value {
		return value.D.Eval(ctx, scope, Continue(func(d Value) Value {
			return cont.Call(Pair{
				A: a,
				D: d,
			}, nil)
		}))
	}))
}

func (value Cons) First() Value {
	return value.A
}

func (value Cons) Rest() Value {
	return value.D
}

var _ Bindable = Cons{}

func (binding Cons) Bind(ctx context.Context, scope *Scope, cont Cont, value Value, _ ...Annotated) ReadyCont {
	return BindList(ctx, scope, cont, binding, value)
}

func (binding Cons) EachBinding(cb func(Symbol, Range) error) error {
	return EachBindingList(binding, cb)
}
