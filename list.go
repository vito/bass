package bass

import (
	"context"
	"fmt"
)

type List interface {
	Value
	Bindable

	First() Value
	Rest() Value
}

func NewList(vals ...Value) List {
	var list List = Empty{}
	for i := len(vals) - 1; i >= 0; i-- {
		list = Pair{
			A: vals[i],
			D: list,
		}
	}

	return list
}

func Each(list List, cb func(Value) error) error {
	for list != (Empty{}) {
		err := cb(list.First())
		if err != nil {
			return err
		}

		err = list.Rest().Decode(&list)
		if err != nil {
			// TODO: better error
			return fmt.Errorf("each: %w", err)
		}
	}

	return nil
}

func ToSlice(list List) ([]Value, error) {
	var vals []Value
	err := Each(list, func(v Value) error {
		vals = append(vals, v)
		return nil
	})
	if err != nil {
		// malformed list
		return nil, fmt.Errorf("to slice: %w", err)
	}

	return vals, nil
}

func IsList(val Value) bool {
	var empty Empty
	err := val.Decode(&empty)
	if err == nil {
		return true
	}

	var list List
	err = val.Decode(&list)
	if err != nil {
		return false
	}

	return IsList(list.Rest())
}

func BindList(ctx context.Context, scope *Scope, cont Cont, binding List, value Value) ReadyCont {
	var e Empty
	if err := value.Decode(&e); err == nil {
		// empty value given for list
		return cont.Call(nil, BindMismatchError{
			Need: binding,
			Have: value,
		})
	}

	var v List
	if err := value.Decode(&v); err != nil {
		// non-list value
		return cont.Call(nil, BindMismatchError{
			Need: binding,
			Have: value,
		})
	}

	var f Bindable
	if err := binding.First().Decode(&f); err != nil {
		return cont.Call(nil, CannotBindError{binding.First()})
	}

	return f.Bind(ctx, scope, Continue(func(fbinding Value) Value { // XXX: fbinding is the binding with evaluated metadata
		var r Bindable
		if err := binding.Rest().Decode(&r); err != nil {
			return cont.Call(nil, CannotBindError{binding.Rest()})
		}

		return r.Bind(ctx, scope, Continue(func(rbinding Value) Value {
			return cont.Call(Pair{fbinding, rbinding}, nil)
		}), v.Rest())
	}), v.First())
}

func EachBindingList(binding List, cb func(Symbol, Range) error) error {
	var f Bindable
	if err := binding.First().Decode(&f); err != nil {
		return CannotBindError{binding.First()}
	}

	if err := f.EachBinding(cb); err != nil {
		return err
	}

	var r Bindable
	if err := binding.Rest().Decode(&r); err != nil {
		return CannotBindError{binding.Rest()}
	}

	return r.EachBinding(cb)
}
