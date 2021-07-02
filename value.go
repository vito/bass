package bass

import (
	"encoding/json"
	"fmt"
	"reflect"
)

type Value interface {
	fmt.Stringer

	Eval(*Env, Cont) ReadyCont

	// Decode coerces and assigns the Value into the given type, analogous to
	// unmarshaling.
	//
	// If the given type is a direct implementor of Value, it must only
	// successfully decode from another instance of that type.
	//
	// If the given type is a Go primitive, it must do its best to coerce into
	// that type. For example, null can Decode into bool, but not Bool.
	Decode(interface{}) error

	// Equal checks whether two values are equal, i.e. same type and equivalent
	// value.
	Equal(Value) bool
}

func ValueOf(src interface{}) (Value, error) {
	switch x := src.(type) {
	case Value:
		return x, nil
	case nil:
		return Null{}, nil
	case bool:
		return Bool(x), nil
	case int:
		return Int(x), nil
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return nil, err
		}

		return Int(i), nil
	case string:
		return String(x), nil
	default:
		rt := reflect.TypeOf(src)
		rv := reflect.ValueOf(src)

		switch rt.Kind() {
		case reflect.Slice:
			return valueOfSlice(rt, rv)
		default:
			return nil, fmt.Errorf("cannot convert %T to Value: %+v", x, x)
		}
	}
}

func valueOfSlice(rt reflect.Type, rv reflect.Value) (Value, error) {
	var list List = Empty{}
	for i := rv.Len() - 1; i >= 0; i-- {
		val, err := ValueOf(rv.Index(i).Interface())
		if err != nil {
			return nil, err
		}

		list = Pair{
			A: val,
			D: list,
		}
	}

	return list, nil
}
