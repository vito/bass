package bass_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
)

func TestBuiltinDecode(t *testing.T) {
	op := bass.Op("noop", "[]", func() {})

	var res bass.Combiner
	err := op.Decode(&res)
	require.NoError(t, err)
	require.Equal(t, op, res)

	var b *bass.Builtin
	err = op.Decode(&b)
	require.NoError(t, err)
	require.Equal(t, op, b)

	app := bass.Func("noop", "[]", func() {})

	err = app.Decode(&res)
	require.NoError(t, err)
	require.Equal(t, app, res)

	err = app.Decode(&b)
	require.Error(t, err)
}

func TestBuiltinEqual(t *testing.T) {
	var val bass.Value = bass.Op("noop", "[]", func() {})
	require.True(t, val.Equal(val))
	require.False(t, val.Equal(bass.Op("noop", "[]", func() {})))

	val = bass.Func("noop", "[]", func() {})
	require.True(t, val.Equal(val))
	require.False(t, val.Equal(bass.Func("noop", "[]", func() {})))
}

func TestBuiltinCall(t *testing.T) {
	type example struct {
		Name string

		Builtin bass.Combiner
		Args    bass.Value

		Result bass.Value
		Err    error
	}

	scope := bass.NewEmptyScope()
	ctx := context.Background()

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
				require.Equal(t, ctx, opCtx)
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
				return errors.New("uh oh")
			}),
			Args: bass.NewList(),
			Err:  errors.New("uh oh"),
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
				return 0, errors.New("uh oh")
			}),
			Args: bass.NewList(),
			Err:  errors.New("uh oh"),
		},
		{
			Name: "multiple arg types",
			Builtin: bass.Func("multi", "[b i s]", func(b bool, i int, s string) []interface{} {
				require.Equal(t, true, b)
				require.Equal(t, 42, i)
				require.Equal(t, "hello", s)
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
			res, err := Call(test.Builtin, scope, test.Args)
			assert.Equal(t, test.Err, err)
			assert.Equal(t, test.Result, res)
		})
	}
}
