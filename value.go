package bass

import (
	"fmt"
	"reflect"
)

type Value interface {
	Decode(interface{}) error

	Eval(*Env) (Value, error)
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
