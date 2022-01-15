package bass

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type Value interface {
	fmt.Stringer

	Eval(context.Context, *Scope, Cont) ReadyCont

	// Equal checks whether two values are equal, i.e. same type and equivalent
	// value.
	Equal(Value) bool

	// Decode coerces and assigns the Value into the given type, analogous to
	// unmarshaling.
	//
	// If the given type is a direct implementor of Value, it must only
	// successfully decode from another instance of that type.
	//
	// If the given type is a Go primitive, it must do its best to coerce into
	// that type. For example, null can Decode into bool, but not Bool.
	//
	// TODO: move this to Encodable/Decodable or something (or rename all this if
	// it's so confusing)
	Decode(interface{}) error
}

// Decodable types typically implement json.Unmarshaler as well.
type Decodable interface {
	FromValue(Value) error
}

// Encodable types typically implement json.Marshaler as well.
type Encodable interface {
	ToValue() Value
}

func ValueOf(src interface{}) (Value, error) {
	switch x := src.(type) {
	case Value:
		return x, nil
	case Encodable:
		return x.ToValue(), nil
	case nil:
		return Null{}, nil
	case bool:
		return Bool(x), nil
	case int:
		return Int(x), nil
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return String(x.String()), nil
		}

		return Int(i), nil
	case string:
		return String(x), nil
	case map[string]interface{}:
		scope := NewEmptyScope()
		for k, v := range x {
			val, err := ValueOf(v)
			if err != nil {
				// TODO: better error
				return nil, err
			}

			scope.Set(SymbolFromJSONKey(k), val)
		}

		return scope, nil
	case map[interface{}]interface{}: // yaml
		scope := NewEmptyScope()
		for k, v := range x {
			s, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("unsupported non-string key (%T): %v", k, k)
			}

			val, err := ValueOf(v)
			if err != nil {
				// TODO: better error
				return nil, err
			}

			scope.Set(SymbolFromJSONKey(s), val)
		}

		return scope, nil
	default:
		rt := reflect.TypeOf(src)
		rv := reflect.ValueOf(src)

		switch rt.Kind() {
		case reflect.Slice:
			return valueOfSlice(rt, rv)
		case reflect.Struct:
			return valueOfStruct(rt, rv)
		default:
			return nil, fmt.Errorf("cannot convert %T to Value: %+v", x, x)
		}
	}
}

func valueOfSlice(_ reflect.Type, rv reflect.Value) (Value, error) {
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

func valueOfStruct(rt reflect.Type, rv reflect.Value) (Value, error) {
	obj := NewEmptyScope()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)

		tag := field.Tag.Get("json")
		segs := strings.Split(tag, ",")
		name := segs[0]
		if name == "" {
			continue
		}

		if isOptional(segs) && rv.Field(i).IsZero() {
			continue
		}

		key := SymbolFromJSONKey(name)

		val, err := ValueOf(rv.Field(i).Interface())
		if err != nil {
			return nil, fmt.Errorf("value of field %s: %w", field.Name, err)
		}

		obj.Set(key, val)
	}

	return obj, nil
}

func Resolve(val Value, r func(Value) (Value, error)) (Value, error) {
	val, err := r(val)
	if err != nil {
		return nil, err
	}

	var list List
	if err := val.Decode(&list); err == nil {
		vals := []Value{}

		err := Each(list, func(val Value) error {
			val, err = Resolve(val, r)
			if err != nil {
				return err
			}
			vals = append(vals, val)
			return nil
		})
		if err != nil {
			return nil, err
		}

		return NewList(vals...), nil
	}

	var scope *Scope
	if err := val.Decode(&scope); err == nil {
		newObj := scope.Copy()

		err := scope.Each(func(k Symbol, v Value) error {
			resolved, err := Resolve(v, r)
			if err != nil {
				return err
			}

			newObj.Set(k, resolved)
			return nil
		})
		if err != nil {
			return nil, err
		}

		return newObj, nil
	}

	return val, nil
}
