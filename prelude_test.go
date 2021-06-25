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
		{
			Name:   "-",
			Bass:   "(- 1 2 3)",
			Result: bass.Int(-4),
		},
		{
			Name:   "- unary",
			Bass:   "(- 1)",
			Result: bass.Int(-1),
		},
		{
			Name:   "* no args",
			Bass:   "(*)",
			Result: bass.Int(1),
		},
		{
			Name:   "* unary",
			Bass:   "(* 5)",
			Result: bass.Int(5),
		},
		{
			Name:   "* product",
			Bass:   "(* 1 2 3 4)",
			Result: bass.Int(24),
		},
		{
			Name:   "max",
			Bass:   "(max 1 3 7 5 4)",
			Result: bass.Int(7),
		},
		{
			Name:   "min",
			Bass:   "(min 5 3 7 2 4)",
			Result: bass.Int(2),
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

	env.Set("operative", operative)

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
				Need: 4,
				Have: 1,
			},
		},
		{
			Name: "invalid op 1",
			Bass: "(op [x])",
			Err: bass.ArityError{
				Name: "op",
				Need: 4,
				Have: 2,
			},
		},
		{
			Name: "invalid op 2",
			Bass: "(op [x] _)",
			Err: bass.ArityError{
				Name: "op",
				Need: 4,
				Have: 3,
			},
		},
		{
			Name: "invalid op 3",
			Bass: "(op . false)",
			Err:  bass.ErrBadSyntax,
		},
		{
			Name: "invalid op 4",
			Bass: "(op [x] . _)",
			Err:  bass.ErrBadSyntax,
		},
		{
			Name:   "wrap",
			Bass:   "((wrap (op x _ x)) 1 2 (+ 1 2))",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			Name:   "unwrap",
			Bass:   "(unwrap (wrap operative))",
			Result: operative,
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

func TestPreludeEnv(t *testing.T) {
	type example struct {
		Name string
		Bass string

		Result   bass.Value
		Bindings bass.Bindings

		Err         error
		ErrContains string
	}

	// used as a test value
	sentinel := bass.String("evaluated")

	for _, test := range []example{
		{
			Name:   "eval",
			Bass:   "((op [x] e (eval x e)) sentinel)",
			Result: bass.String("evaluated"),
		},
		{
			Name:   "def",
			Bass:   "(def foo 1)",
			Result: bass.Symbol("foo"),
			Bindings: bass.Bindings{
				"foo":      bass.Int(1),
				"sentinel": sentinel,
			},
		},
		{
			Name:   "def evaluation",
			Bass:   "(def foo sentinel)",
			Result: bass.Symbol("foo"),
			Bindings: bass.Bindings{
				"foo":      sentinel,
				"sentinel": sentinel,
			},
		},
		{
			Name: "def destructuring",
			Bass: "(def (a . bs) [1 2 3])",
			Result: bass.Pair{
				A: bass.Symbol("a"),
				D: bass.Symbol("bs"),
			},
			Bindings: bass.Bindings{
				"a":        bass.Int(1),
				"bs":       bass.NewList(bass.Int(2), bass.Int(3)),
				"sentinel": sentinel,
			},
		},
		{
			Name: "def destructuring advanced",
			Bass: "(def (a b [c d] e . fs) [1 2 [3 4] 5 6 7])",
			Result: bass.Pair{
				A: bass.Symbol("a"),
				D: bass.Pair{
					A: bass.Symbol("b"),
					D: bass.Pair{
						A: bass.InertPair{
							A: bass.Symbol("c"),
							D: bass.InertPair{
								A: bass.Symbol("d"),
								D: bass.Empty{},
							},
						},
						D: bass.Pair{
							A: bass.Symbol("e"),
							D: bass.Symbol("fs"),
						},
					},
				},
			},
			Bindings: bass.Bindings{
				"a":        bass.Int(1),
				"b":        bass.Int(2),
				"c":        bass.Int(3),
				"d":        bass.Int(4),
				"e":        bass.Int(5),
				"fs":       bass.NewList(bass.Int(6), bass.Int(7)),
				"sentinel": sentinel,
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			reader := bass.NewReader(bytes.NewBufferString(test.Bass))

			val, err := reader.Next()
			require.NoError(t, err)

			env := bass.New()
			env.Set("sentinel", sentinel)

			res, err := val.Eval(env)
			if test.Err != nil {
				require.Equal(t, test.Err, err)
			} else if test.ErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.ErrContains)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.Result, res)

				if test.Bindings != nil {
					require.Equal(t, test.Bindings, env.Bindings)
				}
			}
		})
	}
}

func TestPreludeBoolean(t *testing.T) {
	type example struct {
		Name string
		Bass string

		Result   bass.Value
		Bindings bass.Bindings

		Err         error
		ErrContains string
	}

	// used as a test value
	sentinel := bass.String("evaluated")

	for _, test := range []example{
		{
			Name:   "if true",
			Bass:   "(if true sentinel unevaluated)",
			Result: sentinel,
		},
		{
			Name:   "if false",
			Bass:   "(if false unevaluated sentinel)",
			Result: sentinel,
		},
		{
			Name:   "if null",
			Bass:   "(if null unevaluated sentinel)",
			Result: sentinel,
		},
		{
			Name:   "if empty",
			Bass:   "(if [] sentinel unevaluated)",
			Result: sentinel,
		},
		{
			Name:   "if string",
			Bass:   `(if "" sentinel unevaluated)`,
			Result: sentinel,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			reader := bass.NewReader(bytes.NewBufferString(test.Bass))

			val, err := reader.Next()
			require.NoError(t, err)

			env := bass.New()
			env.Set("sentinel", sentinel)

			res, err := val.Eval(env)
			if test.Err != nil {
				require.Equal(t, test.Err, err)
			} else if test.ErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.ErrContains)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.Result, res)

				if test.Bindings != nil {
					require.Equal(t, test.Bindings, env.Bindings)
				}
			}
		})
	}
}
