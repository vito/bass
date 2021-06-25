package bass

import (
	"fmt"
	"reflect"
)

type Builtin struct {
	Name string
	Func reflect.Value
}

func (builtin Builtin) Decode(dest interface{}) error {
	// TODO: assign to *Builtin?
	return fmt.Errorf("Builtin.Decode is not implemented")
}

func Func(name string, f interface{}) *Builtin {
	fun := reflect.ValueOf(f)
	if fun.Kind() != reflect.Func {
		panic("Func takes a func()")
	}

	return &Builtin{
		Name: name,
		Func: fun,
	}
}

var valType = reflect.TypeOf((*Value)(nil)).Elem()
var errType = reflect.TypeOf((*error)(nil)).Elem()

func (builtin Builtin) Call(val Value, env *Env) (Value, error) {
	eargs, err := val.Eval(env)
	if err != nil {
		return nil, err
	}

	list, ok := eargs.(List)
	if !ok {
		return nil, fmt.Errorf("builtin functions must be applied to lists")
	}

	args := []Value{}
	for list != (Empty{}) {
		args = append(args, list.First())

		list, ok = list.Rest().(List)
		if !ok {
			return nil, fmt.Errorf("nani?!")
		}
	}

	ftype := builtin.Func.Type()

	argc := ftype.NumIn()
	if ftype.IsVariadic() {
		argc--

		if len(args) < argc {
			return nil, ArityError{
				Name:     builtin.Name,
				Need:     argc,
				Have:     len(args),
				Variadic: true,
			}
		}
	} else if len(args) != argc {
		return nil, ArityError{
			Name: builtin.Name,
			Need: argc,
			Have: len(args),
		}
	}

	fargs := []reflect.Value{}

	for i := 0; i < argc; i++ {
		t := ftype.In(i)

		dest := reflect.New(t)
		if t == valType { // pass Value with no conversion
			dest.Elem().Set(reflect.ValueOf(args[i]))
		} else {
			err := args[i].Decode(dest.Interface())
			if err != nil {
				return nil, err
			}
		}

		fargs = append(fargs, dest.Elem())
	}

	if ftype.IsVariadic() {
		variadic := args[argc:]
		variadicType := ftype.In(argc)

		subType := variadicType.Elem()
		for _, varg := range variadic {
			dest := reflect.New(subType)
			if subType == valType { // pass Value with no conversion
				dest.Elem().Set(reflect.ValueOf(varg))
			} else {
				err := varg.Decode(dest.Interface())
				if err != nil {
					return nil, err
				}
			}

			fargs = append(fargs, dest.Elem())
		}
	}

	result := builtin.Func.Call(fargs)

	switch ftype.NumOut() {
	case 0:
		return Null{}, nil
	case 1:
		if ftype.Out(0) == errType {
			if !result[0].IsNil() {
				return nil, result[0].Interface().(error)
			}

			return Null{}, nil
		}

		return ValueOf(result[0].Interface())
	case 2:
		if ftype.Out(1) != errType {
			return nil, fmt.Errorf("multiple return values are not supported")
		}

		if !result[1].IsNil() {
			return nil, result[1].Interface().(error)
		}

		return ValueOf(result[0].Interface())
	}

	return nil, nil
}
