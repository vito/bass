package bass_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

var pair = bass.Pair{
	A: bass.Int(1),
	D: bass.Empty{},
}

var env = bass.NewEnv()

type Const struct {
	bass.Value
}

func (value Const) Eval(*bass.Env) (bass.Value, error) {
	return value.Value, nil
}

var sym = Const{
	Value: bass.Symbol("sym"),
}

var zeroes = []bass.Value{
	bass.Null{},
	bass.Bool(true),
	bass.Bool(false),
	bass.Empty{},
	pair,
	bass.Int(0),
	bass.String("str"),
	sym,
	env,
}

func TestPreludePrimitivePredicates(t *testing.T) {
	env := bass.New()

	type example struct {
		Name   string
		Trues  []bass.Value
		Falses []bass.Value
	}

	for _, test := range []example{
		{
			Name: "null?",
			Trues: []bass.Value{
				bass.Null{},
			},
			Falses: []bass.Value{
				bass.Bool(false),
				pair,
				bass.Empty{},
				bass.Int(0),
				bass.String(""),
			},
		},
		{
			Name: "boolean?",
			Trues: []bass.Value{
				bass.Bool(true),
				bass.Bool(false),
			},
			Falses: []bass.Value{
				bass.Int(1),
				bass.String("true"),
			},
		},
		{
			Name: "number?",
			Trues: []bass.Value{
				bass.Int(0),
			},
			Falses: []bass.Value{
				bass.Bool(true),
				bass.String("1"),
			},
		},
		{
			Name: "string?",
			Trues: []bass.Value{
				bass.String("str"),
			},
			Falses: []bass.Value{
				Const{bass.Symbol("1")},
				bass.Empty{},
			},
		},
		{
			Name: "symbol?",
			Trues: []bass.Value{
				sym,
			},
			Falses: []bass.Value{
				bass.String("str"),
			},
		},
		{
			Name: "empty?",
			Trues: []bass.Value{
				bass.Null{},
				bass.Empty{},
				bass.String(""),
			},
			Falses: []bass.Value{
				bass.Bool(false),
			},
		},
		{
			Name: "pair?",
			Trues: []bass.Value{
				pair,
			},
			Falses: []bass.Value{
				bass.Empty{},
				bass.Null{},
			},
		},
		{
			Name: "list?",
			Trues: []bass.Value{
				bass.Empty{},
				pair,
			},
			Falses: []bass.Value{
				bass.Null{},
				bass.String(""),
			},
		},
		{
			Name: "env?",
			Trues: []bass.Value{
				env,
			},
			Falses: []bass.Value{
				pair,
			},
		},
		{
			Name: "combiner?",
			Trues: []bass.Value{
				bass.Op("quote", func(args bass.List, env *bass.Env) bass.Value {
					return args.First()
				}),
			},
		},
		{
			Name: "applicative?",
			Trues: []bass.Value{
				bass.Func("id", func(val bass.Value) bass.Value {
					return val
				}),
			},
			Falses: []bass.Value{
				bass.Op("quote", func(args bass.List, env *bass.Env) bass.Value {
					return args.First()
				}),
			},
		},
		{
			Name: "operative?",
			Trues: []bass.Value{
				bass.Op("quote", func(args bass.List, env *bass.Env) bass.Value {
					return args.First()
				}),
			},
			Falses: []bass.Value{
				bass.Func("id", func(val bass.Value) bass.Value {
					return val
				}),
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			// if test.Bass != "" {
			// 	reader := bass.NewReader(bytes.NewBufferString(test.Bass))

			// 	val, err := reader.Next()
			// 	require.NoError(t, err)

			// 	res, err := val.Eval(env)
			// 	require.NoError(t, err)

			// 	require.Equal(t, test.Result, res)
			// } else {
			for _, arg := range test.Trues {
				t.Run(fmt.Sprintf("%v", arg), func(t *testing.T) {
					res, err := bass.Apply{
						A: bass.Symbol(test.Name),
						D: bass.NewList(arg),
					}.Eval(env)
					require.NoError(t, err)
					require.Equal(t, bass.Bool(true), res)
				})
			}

			for _, arg := range test.Falses {
				t.Run(fmt.Sprintf("%v", arg), func(t *testing.T) {
					res, err := bass.Apply{
						A: bass.Symbol(test.Name),
						D: bass.NewList(arg),
					}.Eval(env)
					require.NoError(t, err)
					require.Equal(t, bass.Bool(false), res)
				})
			}
			// }
		})
	}
}

func TestPreludeMath(t *testing.T) {
	env := bass.New()

	type example struct {
		Name   string
		Bass   string
		Result bass.Value
	}

	for _, test := range []example{
		{
			Name:   "+",
			Bass:   "(+ 1 2 3)",
			Result: bass.Int(6),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			reader := bass.NewReader(bytes.NewBufferString(test.Bass))

			val, err := reader.Next()
			require.NoError(t, err)

			res, err := val.Eval(env)
			require.NoError(t, err)

			require.Equal(t, test.Result, res)
		})
	}
}
