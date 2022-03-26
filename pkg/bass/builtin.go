package bass

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
)

type Builtin struct {
	Name      string
	Formals   Value
	Func      reflect.Value
	Operative bool
}

var _ Value = (*Builtin)(nil)

func (value *Builtin) Equal(other Value) bool {
	var o *Builtin
	return other.Decode(&o) == nil && value == o
}

func (value *Builtin) String() string {
	return fmt.Sprintf("<builtin op: %s>", Pair{
		A: Symbol(value.Name),
		D: value.Formals,
	})
}

func (value *Builtin) Decode(dest any) error {
	switch x := dest.(type) {
	case **Builtin:
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

func (value *Builtin) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

func (value *Builtin) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func Op(name, signature string, f any) *Builtin {
	fun := reflect.ValueOf(f)
	if fun.Kind() != reflect.Func {
		panic("Op takes a func()")
	}

	reader := NewReader(bytes.NewBufferString(signature), name)

	formals, err := reader.Next()
	if err != nil {
		panic(err)
	}

	return &Builtin{
		Name:      name,
		Formals:   formals,
		Func:      fun,
		Operative: true,
	}
}

func Func(name, formals string, f any) Combiner {
	op := Op(name, formals, f)
	op.Operative = false
	return Wrap(op)
}

var valType = reflect.TypeOf((*Value)(nil)).Elem()
var errType = reflect.TypeOf((*error)(nil)).Elem()
var ctxType = reflect.TypeOf((*context.Context)(nil)).Elem()
var contType = reflect.TypeOf((*ReadyCont)(nil)).Elem()

func (builtin Builtin) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	ftype := builtin.Func.Type()

	fargs := []reflect.Value{}

	autoArgs := 0

	// needs context.Context
	if ftype.NumIn() >= 1 && ftype.In(0) == ctxType {
		fargs = append(fargs, reflect.ValueOf(ctx))
		autoArgs++
	}

	// needs Cont
	if ftype.NumOut() == 1 && ftype.Out(0) == contType {
		fargs = append(fargs, reflect.ValueOf(cont))
		autoArgs++
	}

	// needs Scope
	if builtin.Operative {
		fargs = append(fargs, reflect.ValueOf(scope))
		autoArgs++
	}

	var list List
	err := val.Decode(&list)
	if err != nil {
		return cont.Call(nil, ErrBadSyntax)
	}

	argc := autoArgs
	givenArgs := []Value{}
	err = Each(list, func(v Value) error {
		givenArgs = append(givenArgs, v)
		argc++
		return nil
	})
	if err != nil {
		return cont.Call(nil, ErrBadSyntax)
	}

	fargc := ftype.NumIn()

	if ftype.IsVariadic() {
		fargc--
		// argc--

		if argc < fargc {
			return cont.Call(nil, ArityError{
				Name:     builtin.Name,
				Need:     fargc,
				Have:     argc,
				Variadic: true,
			})
		}
	} else if argc != fargc {
		return cont.Call(nil, ArityError{
			Name: builtin.Name,
			Need: fargc,
			Have: argc,
		})
	}

	for i, arg := range givenArgs {
		farg := autoArgs + i
		t := ftype.In(farg)

		if ftype.IsVariadic() && farg == fargc {
			variadic := givenArgs[i:]
			subType := t.Elem()
			for _, varg := range variadic {
				dest := reflect.New(subType)
				err := varg.Decode(dest.Interface())
				if err != nil {
					return cont.Call(nil, fmt.Errorf("%s decode variadic arg: %w", builtin.Name, err))
				}

				fargs = append(fargs, dest.Elem())
			}

			break
		}

		dest := reflect.New(t)
		err := arg.Decode(dest.Interface())
		if err != nil {
			return cont.Call(nil, fmt.Errorf("%s decode arg[%d]: %w", builtin.Name, i, err))
		}

		fargs = append(fargs, dest.Elem())
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

		res, err := ValueOf(result[0].Interface())
		if err != nil {
			return cont.Call(nil, err)
		}

		var rdy ReadyCont
		if err := res.Decode(&rdy); err == nil {
			return rdy
		}

		return cont.Call(res, nil)
	case 2:
		if ftype.Out(1) != errType {
			return cont.Call(nil, fmt.Errorf("multiple return values are not supported"))
		}

		if !result[1].IsNil() {
			return cont.Call(nil, result[1].Interface().(error))
		}

		res, err := ValueOf(result[0].Interface())
		if err != nil {
			return cont.Call(nil, err)
		}

		var rdy ReadyCont
		if err := res.Decode(&rdy); err == nil {
			return rdy
		}

		return cont.Call(res, nil)
	default:
		return cont.Call(nil, fmt.Errorf("builtins can return 0, 1, or 2 values, have %d", ftype.NumOut()))
	}
}
