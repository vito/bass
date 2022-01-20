package bass_test

import (
	"context"
	"errors"
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
)

func TestBuiltinDecode(t *testing.T) {
	is := is.New(t)

	op := bass.Op("noop", "[]", func() {})

	var res bass.Combiner
	err := op.Decode(&res)
	is.NoErr(err)
	is.Equal(res, op)

	var b *bass.Builtin
	err = op.Decode(&b)
	is.NoErr(err)
	is.Equal(b, op)

	app := bass.Func("noop", "[]", func() {})

	err = app.Decode(&res)
	is.NoErr(err)
	is.Equal(res, app)

	err = app.Decode(&b)
	is.True(err != nil)
}

func TestBuiltinEqual(t *testing.T) {
	is := is.New(t)

	var val bass.Value = bass.Op("noop", "[]", func() {})
	Equal(t, val, val)
	is.True(!val.Equal(bass.Op("noop", "[]", func() {})))

	val = bass.Func("noop", "[]", func() {})
	Equal(t, val, val)
	is.True(!val.Equal(bass.Func("noop", "[]", func() {})))
}

func TestBuiltinCall(t *testing.T) {
	is := is.New(t)

	type example struct {
		Name string

		Builtin bass.Combiner
		Args    bass.Value

		Result bass.Value
		Err    error
	}

	scope := bass.NewEmptyScope()
	ctx := context.Background()

	uhOh := errors.New("uh oh")
	for _, test := range []example{
		{
			Name: "operative args",
			Builtin: bass.Op("foo", "[sym]", func(scope *bass.Scope, arg bass.Symbol) bass.Value {
				return arg
			}),
			Args:   bass.NewList(bass.Symbol("sym")),
			Result: bass.Symbol("sym"),
		},
		{
			Name: "operative scope",
			Builtin: bass.Op("foo", "[sym]", func(scope *bass.Scope, _ bass.Symbol) bass.Value {
				return scope
			}),
			Args:   bass.NewList(bass.Symbol("sym")),
			Result: scope,
		},
		{
			Name: "operative cont",
			Builtin: bass.Op("foo", "[sym]", func(cont bass.Cont, scope *bass.Scope, _ bass.Symbol) bass.ReadyCont {
				return cont.Call(bass.Int(42), nil)
			}),
			Args:   bass.NewList(bass.Symbol("sym")),
			Result: bass.Int(42),
		},
		{
			Name: "operative ctx",
			Builtin: bass.Op("foo", "[sym]", func(opCtx context.Context, scope *bass.Scope, arg bass.Symbol) bass.Value {
				is.Equal(opCtx, ctx)
				return arg
			}),
			Args:   bass.NewList(bass.Symbol("sym")),
			Result: bass.Symbol("sym"),
		},
		{
			Name:    "no return",
			Builtin: bass.Func("noop", "[]", func() {}),
			Args:    bass.NewList(),
			Result:  bass.Null{},
		},
		{
			Name: "nil error",
			Builtin: bass.Func("noop", "[]", func() error {
				return nil
			}),
			Args:   bass.NewList(),
			Result: bass.Null{},
		},
		{
			Name: "non-nil error",
			Builtin: bass.Func("noop fail", "[]", func() error {
				return uhOh
			}),
			Args: bass.NewList(),
			Err:  uhOh,
		},
		{
			Name: "no conversion",
			Builtin: bass.Func("id", "[val]", func(v bass.Value) bass.Value {
				return v
			}),
			Args:   bass.NewList(bass.Int(42)),
			Result: bass.Int(42),
		},
		{
			Name: "int conversion",
			Builtin: bass.Func("inc", "[num]", func(v int) int {
				return v + 1
			}),
			Args:   bass.NewList(bass.Int(42)),
			Result: bass.Int(43),
		},
		{
			Name: "variadic",
			Builtin: bass.Func("+", "nums", func(vs ...int) int {
				sum := 0
				for _, v := range vs {
					sum += v
				}

				return sum
			}),
			Args:   bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
			Result: bass.Int(6),
		},
		{
			Name: "value, no error",
			Builtin: bass.Func("value", "[]", func() (int, error) {
				return 42, nil
			}),
			Args:   bass.NewList(),
			Result: bass.Int(42),
		},
		{
			Name: "value, error",
			Builtin: bass.Func("value err", "[]", func() (int, error) {
				return 0, uhOh
			}),
			Args: bass.NewList(),
			Err:  uhOh,
		},
		{
			Name: "multiple arg types",
			Builtin: bass.Func("multi", "[b i s]", func(b bool, i int, s string) []interface{} {
				is.Equal(b, true)
				is.Equal(i, 42)
				is.Equal(s, "hello")
				return []interface{}{s, i, b}
			}),
			Args:   bass.NewList(bass.Bool(true), bass.Int(42), bass.String("hello")),
			Result: bass.NewList(bass.String("hello"), bass.Int(42), bass.Bool(true)),
		},
		{
			Name: "arity expect 0 get 1",
			Builtin: bass.Func("noop", "[]", func() error {
				return nil
			}),
			Args: bass.NewList(bass.Int(42)),
			Err: bass.ArityError{
				Name: "noop",
				Need: 0,
				Have: 1,
			},
		},
		{
			Name: "arity expect 1 get 0",
			Builtin: bass.Func("id", "[val]", func(v bass.Value) bass.Value {
				return v
			}),
			Args: bass.NewList(),
			Err: bass.ArityError{
				Name: "id",
				Need: 1,
				Have: 0,
			},
		},
		{
			Name: "arity expect 1 get 2",
			Builtin: bass.Func("id", "[val]", func(v bass.Value) bass.Value {
				return v
			}),
			Args: bass.NewList(bass.Int(42), bass.String("hello")),
			Err: bass.ArityError{
				Name: "id",
				Need: 1,
				Have: 2,
			},
		},
		{
			Name: "arity expect at least 1 get 3",
			Builtin: bass.Func("var", "[num & rest]", func(i int, vs ...bass.Value) int {
				return i + len(vs)
			}),
			Args:   bass.NewList(bass.Int(42), bass.String("hello"), bass.String("world")),
			Result: bass.Int(44),
		},
		{
			Name: "arity expect at least 1 get 0",
			Builtin: bass.Func("var", "[num & res]", func(i int, vs ...bass.Value) int {
				return i + len(vs)
			}),
			Args: bass.NewList(),
			Err: bass.ArityError{
				Name:     "var",
				Need:     1,
				Variadic: true,
				Have:     0,
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			is := is.New(t)
			res, err := Call(test.Builtin, scope, test.Args)
			is.True(errors.Is(err, test.Err))
			is.Equal(res, test.Result)
		})
	}
}
