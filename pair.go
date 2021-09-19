package bass

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

type Pair struct {
	A Value
	D Value
}

var _ Value = Pair{}

func (value Pair) String() string {
	return formatList(value, "(", ")")
}

func (value Pair) MarshalJSON() ([]byte, error) {
	slice, err := ToSlice(value)
	if err != nil {
		return nil, EncodeError{value}
	}

	return json.Marshal(slice)
}

func (value *Pair) UnmarshalJSON(payload []byte) error {
	var x interface{}
	err := UnmarshalJSON(payload, &x)
	if err != nil {
		return err
	}

	val, err := ValueOf(x)
	if err != nil {
		return err
	}

	obj, ok := val.(Pair)
	if !ok {
		return fmt.Errorf("expected Pair from ValueOf, got %T", val)
	}

	*value = obj

	return nil
}

func (value Pair) Equal(other Value) bool {
	var o Pair
	if err := other.Decode(&o); err != nil {
		return false
	}

	return value.A.Equal(o.A) && value.D.Equal(o.D)
}

func (value Pair) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Pair:
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
		return decodeSlice(value, dest)
	}
}

var _ List = Pair{}

func (value Pair) First() Value {
	return value.A
}

func (value Pair) Rest() Value {
	return value.D
}

// Pair combines the first operand with the second operand.
//
// If the first value is not a Combiner, an error is returned.
func (value Pair) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return value.A.Eval(ctx, scope, Continue(func(f Value) Value {
		var combiner Combiner
		err := f.Decode(&combiner)
		if err != nil {
			return cont.Call(nil, fmt.Errorf("apply %s: %w", f, err))
		}

		return combiner.Call(ctx, value.D, scope, cont)
	}))
}

var _ Bindable = Pair{}

func (binding Pair) Bind(scope *Scope, value Value, _ ...Annotated) error {
	return BindList(binding, scope, value)
}

func (binding Pair) EachBinding(cb func(Symbol, Range) error) error {
	return EachBindingList(binding, cb)
}

func formatList(list List, odelim, cdelim string) string {
	out := odelim

	for list != (Empty{}) {
		out += list.First().String()

		var empty Empty
		err := list.Rest().Decode(&empty)
		if err == nil {
			break
		}

		err = list.Rest().Decode(&list)
		if err != nil {
			out += " & "
			out += list.Rest().String()
			break
		}

		out += " "
	}

	out += cdelim

	return out
}

func decodeSlice(value List, dest interface{}) error {
	rt := reflect.TypeOf(dest)
	if rt.Kind() != reflect.Ptr {
		return fmt.Errorf("decode into non-pointer %T", dest)
	}

	re := rt.Elem()
	rv := reflect.ValueOf(dest).Elem()

	if re.Kind() != reflect.Slice {
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}

	slice, err := ToSlice(value)
	if err != nil {
		return err
	}

	rs := reflect.MakeSlice(re, len(slice), len(slice))
	for i, v := range slice {
		err := v.Decode(rs.Index(i).Addr().Interface())
		if err != nil {
			return err
		}
	}

	rv.Set(rs)

	return nil
}
