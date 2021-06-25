package bass_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

var operative = &bass.Operative{
	Formals: bass.NewList(bass.Symbol("form")),
	Eformal: bass.Symbol("env"),
	Body: bass.InertPair{
		A: bass.Symbol("form"),
		D: bass.Symbol("env"),
	},
}

var pair = Const{
	bass.Pair{
		A: bass.Int(1),
		D: bass.Empty{},
	},
}

var inertPair = bass.InertPair{
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
				inertPair,
				bass.Empty{},
				bass.Ignore{},
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
				bass.Ignore{},
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
				bass.Ignore{},
			},
		},
		{
			Name: "pair?",
			Trues: []bass.Value{
				pair,
				inertPair,
			},
			Falses: []bass.Value{
				bass.Empty{},
				bass.Ignore{},
				bass.Null{},
			},
		},
		{
			Name: "list?",
			Trues: []bass.Value{
				bass.Empty{},
				pair,
				inertPair,
			},
			Falses: []bass.Value{
				bass.Ignore{},
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
				operative,
			},
			Falses: []bass.Value{
				bass.Func("id", func(val bass.Value) bass.Value {
					return val
				}),
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			for _, arg := range test.Trues {
				t.Run(fmt.Sprintf("%v", arg), func(t *testing.T) {
					res, err := bass.Pair{
						A: bass.Symbol(test.Name),
						D: bass.NewList(arg),
					}.Eval(env)
					require.NoError(t, err)
					require.Equal(t, bass.Bool(true), res)
				})
			}

			for _, arg := range test.Falses {
				t.Run(fmt.Sprintf("%v", arg), func(t *testing.T) {
					res, err := bass.Pair{
						A: bass.Symbol(test.Name),
						D: bass.NewList(arg),
					}.Eval(env)
					require.NoError(t, err)
					require.Equal(t, bass.Bool(false), res)
				})
			}
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

func TestPreludeConstructors(t *testing.T) {
	env := bass.New()

	type example struct {
		Name string
		Bass string

		Result      bass.Value
		Err         error
		ErrContains string
	}

	for _, test := range []example{
		{
			Name: "cons",
			Bass: "(cons 1 2)",
			Result: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
		},
		{
			Name: "op",
			Bass: "(op (x) e [x e])",
			Result: &bass.Operative{
				Formals: bass.NewList(bass.Symbol("x")),
				Eformal: bass.Symbol("e"),
				Body:    bass.NewInertList(bass.Symbol("x"), bass.Symbol("e")),
				Env:     env,
			},
		},
		{
			Name: "bracket op",
			Bass: "(op [x] e [x e])",
			Result: &bass.Operative{
				Formals: bass.NewInertList(bass.Symbol("x")),
				Eformal: bass.Symbol("e"),
				Body:    bass.NewInertList(bass.Symbol("x"), bass.Symbol("e")),
				Env:     env,
			},
		},
		{
			Name: "invalid op 0",
			Bass: "(op)",
			Err: bass.ArityError{
				Name: "op",
				Need: 3,
				Have: 0,
			},
		},
		{
			Name: "invalid op 1",
			Bass: "(op [x])",
			Err: bass.ArityError{
				Name: "op",
				Need: 3,
				Have: 1,
			},
		},
		{
			Name: "invalid op 2",
			Bass: "(op [x] _)",
			Err: bass.ArityError{
				Name: "op",
				Need: 3,
				Have: 2,
			},
		},
		{
			Name:        "invalid op 3",
			Bass:        "(op . false)",
			ErrContains: "cannot decode bass.Bool into *bass.List",
		},
		{
			Name: "invalid op 4",
			Bass: "(op [x] . _)",
			Err:  bass.ErrBadSyntax,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			reader := bass.NewReader(bytes.NewBufferString(test.Bass))

			val, err := reader.Next()
			require.NoError(t, err)

			res, err := val.Eval(env)
			if test.Err != nil {
				require.Equal(t, test.Err, err)
			} else if test.ErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.ErrContains)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.Result, res)
			}
		})
	}
}
