package bass

import (
	"fmt"
	"reflect"
)

type Builtin struct {
	Name      string
	Func      reflect.Value
	Operative bool
}

var _ Value = (*Builtin)(nil)

func (value *Builtin) Equal(other Value) bool {
	var o *Builtin
	return other.Decode(&o) == nil && value == o
}

func (value *Builtin) String() string {
	return fmt.Sprintf("<builtin op: %s>", value.Name)
}

func (value *Builtin) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case **Builtin:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value *Builtin) Eval(env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func Op(name string, f interface{}) *Builtin {
	fun := reflect.ValueOf(f)
	if fun.Kind() != reflect.Func {
		panic("Op takes a func()")
	}

	return &Builtin{
		Name:      name,
		Func:      fun,
		Operative: true,
	}
}

func Func(name string, f interface{}) Combiner {
	op := Op(name, f)
	op.Operative = false
	return Applicative{op}
}

var valType = reflect.TypeOf((*Value)(nil)).Elem()
var errType = reflect.TypeOf((*error)(nil)).Elem()
var contType = reflect.TypeOf((*ReadyCont)(nil)).Elem()

func (builtin Builtin) Call(val Value, env *Env, cont Cont) ReadyCont {
	args := []interface{}{}
	if builtin.Operative {
		args = append(args, cont, env)
	}

	var list List
	err := val.Decode(&list)
	if err != nil {
		return cont.Call(nil, ErrBadSyntax)
	}

	for list != (Empty{}) {
		args = append(args, list.First())

		err = list.Rest().Decode(&list)
		if err != nil {
			return cont.Call(nil, ErrBadSyntax)
		}
	}

	ftype := builtin.Func.Type()

	argc := ftype.NumIn()
	if ftype.IsVariadic() {
		argc--

		if len(args) < argc {
			return cont.Call(nil, ArityError{
				Name:     builtin.Name,
				Need:     argc,
				Have:     len(args),
				Variadic: true,
			})
		}
	} else if len(args) != argc {
		return cont.Call(nil, ArityError{
			Name: builtin.Name,
			Need: argc,
			Have: len(args),
		})
	}

	fargs := []reflect.Value{}

	for i := 0; i < argc; i++ {
		t := ftype.In(i)

		dest := reflect.New(t)
		if reflect.TypeOf(args[i]).AssignableTo(t) {
			dest.Elem().Set(reflect.ValueOf(args[i]))
		} else {
			// XXX: this *should* be safe, but it's a little smelly. the only
			// non-Value type is Cont, statically appended above for Op builtins.
			err = args[i].(Value).Decode(dest.Interface())
			if err != nil {
				return cont.Call(nil, err)
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
			if reflect.TypeOf(varg).AssignableTo(subType) {
				dest.Elem().Set(reflect.ValueOf(varg))
			} else {
				// XXX: this *should* be safe, but it's a little smelly. the only
				// non-Value type is Cont, statically appended above for Op builtins.
				err := varg.(Value).Decode(dest.Interface())
				if err != nil {
					return cont.Call(nil, err)
				}
			}

			fargs = append(fargs, dest.Elem())
		}
	}

	result := builtin.Func.Call(fargs)

	switch ftype.NumOut() {
	case 0:
		return cont.Call(Null{}, nil)
	case 1:
		if ftype.Out(0) == errType {
			if !result[0].IsNil() {
				return cont.Call(nil, result[0].Interface().(error))
			}

			return cont.Call(Null{}, nil)
		}

		if ftype.Out(0) == contType {
			return result[0].Interface().(ReadyCont)
		}

		res, err := ValueOf(result[0].Interface())
		if err != nil {
			return cont.Call(nil, err)
		}

		return cont.Call(res, nil)
	case 2:
		if ftype.Out(1) != errType {
			return cont.Call(nil, fmt.Errorf("multiple return values are not supported"))
		}

		if !result[1].IsNil() {
			return cont.Call(nil, result[1].Interface().(error))
		}

		if ftype.Out(0) == contType {
			return result[0].Interface().(ReadyCont)
		}

		res, err := ValueOf(result[0].Interface())
		if err != nil {
			return cont.Call(nil, err)
		}

		return cont.Call(res, nil)
	default:
		return cont.Call(nil, fmt.Errorf("builtins can return 0, 1, or 2 values, have %d", ftype.NumOut()))
	}
}
