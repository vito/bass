package bass_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mattn/go-colorable"
	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

var docsOut = new(bytes.Buffer)

func init() {
	bass.DocsWriter = colorable.NewNonColorable(docsOut)
}

var operative = &bass.Operative{
	Formals: bass.NewList(bass.Symbol("form")),
	Eformal: bass.Symbol("env"),
	Body: bass.Cons{
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

var nonListPair = Const{
	bass.Pair{
		A: bass.Int(1),
		D: bass.Int(2),
	},
}

var cons = bass.Cons{
	A: bass.Int(1),
	D: bass.Empty{},
}

var object = bass.Object{
	"a": bass.Int(1),
	"b": bass.Bool(true),
}

var assoc = bass.Assoc{
	{bass.Keyword("a"), bass.Int(1)},
	{bass.Keyword("b"), bass.Bool(true)},
}

var sym = Const{
	Value: bass.Symbol("sym"),
}

type BasicExample struct {
	Name string
	Bass string

	Result      bass.Value
	Err         error
	ErrContains string
}

func (example BasicExample) Run(t *testing.T) {
	t.Run(example.Name, func(t *testing.T) {
		env := bass.NewStandardEnv()

		reader := bytes.NewBufferString(example.Bass)

		res, err := bass.EvalReader(env, reader)
		if example.Err != nil {
			require.ErrorIs(t, err, example.Err)
		} else if example.ErrContains != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), example.ErrContains)
		} else {
			require.NoError(t, err)
			Equal(t, res, example.Result)
		}
	})
}

func TestGroundPrimitivePredicates(t *testing.T) {
	env := bass.NewStandardEnv()

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
				cons,
				bass.Empty{},
				bass.Ignore{},
				bass.Int(0),
				bass.String(""),
			},
		},
		{
			Name: "ignore?",
			Trues: []bass.Value{
				bass.Ignore{},
			},
			Falses: []bass.Value{
				bass.Bool(false),
				pair,
				cons,
				bass.Empty{},
				bass.Null{},
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
				bass.Null{},
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
				bass.Empty{},
				bass.Ignore{},
				sym,
				bass.Keyword("key"),
			},
		},
		{
			Name: "symbol?",
			Trues: []bass.Value{
				sym,
			},
			Falses: []bass.Value{
				bass.String("str"),
				bass.Keyword("key"),
			},
		},
		{
			Name: "keyword?",
			Trues: []bass.Value{
				bass.Keyword("key"),
			},
			Falses: []bass.Value{
				sym,
				bass.String("str"),
			},
		},
		{
			Name: "empty?",
			Trues: []bass.Value{
				bass.Object{},
				bass.Assoc{},
				bass.Null{},
				bass.Empty{},
				bass.String(""),
			},
			Falses: []bass.Value{
				bass.Bool(false),
				bass.Ignore{},
				object,
				assoc,
			},
		},
		{
			Name: "pair?",
			Trues: []bass.Value{
				pair,
				cons,
			},
			Falses: []bass.Value{
				bass.Empty{},
				bass.Ignore{},
				bass.Null{},
				object,
				assoc,
			},
		},
		{
			Name: "list?",
			Trues: []bass.Value{
				bass.Empty{},
				pair,
				cons,
			},
			Falses: []bass.Value{
				nonListPair,
				bass.Ignore{},
				bass.Null{},
				bass.String(""),
				object,
				assoc,
			},
		},
		{
			Name: "object?",
			Trues: []bass.Value{
				object,
				assoc,
			},
			Falses: []bass.Value{
				bass.Empty{},
				bass.Ignore{},
				bass.Null{},
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
			Name: "sink?",
			Trues: []bass.Value{
				bass.Stdout,
			},
			Falses: []bass.Value{
				bass.Stdin,
			},
		},
		{
			Name: "source?",
			Trues: []bass.Value{
				bass.Stdin,
			},
			Falses: []bass.Value{
				bass.Stdout,
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
		{
			Name: "path?",
			Trues: []bass.Value{
				bass.DirectoryPath{"foo"},
				bass.FilePath{"foo"},
				bass.CommandPath{"foo"},
			},
			Falses: []bass.Value{
				bass.String("foo"),
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			for _, arg := range test.Trues {
				t.Run(fmt.Sprintf("%v", arg), func(t *testing.T) {
					res, err := Eval(env, bass.Pair{
						A: bass.Symbol(test.Name),
						D: bass.NewList(arg),
					})
					require.NoError(t, err)
					require.Equal(t, bass.Bool(true), res)
				})
			}

			for _, arg := range test.Falses {
				t.Run(fmt.Sprintf("%v", arg), func(t *testing.T) {
					res, err := Eval(env, bass.Pair{
						A: bass.Symbol(test.Name),
						D: bass.NewList(arg),
					})
					require.NoError(t, err)
					require.Equal(t, bass.Bool(false), res)
				})
			}
		})
	}
}

func TestGroundNumeric(t *testing.T) {
	for _, test := range []BasicExample{
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
		{
			Name:   "min",
			Bass:   "(min 5 3 7 2 4)",
			Result: bass.Int(2),
		},
	} {
		test.Run(t)
	}
}

func TestGroundComparison(t *testing.T) {
	for _, test := range []BasicExample{
		{
			Name:   "= null",
			Bass:   "(= null null)",
			Result: bass.Bool(true),
		},
		{
			Name:   "= null empty",
			Bass:   "(= null [])",
			Result: bass.Bool(false),
		},
		{
			Name:   "= null ignore",
			Bass:   "(= null _)",
			Result: bass.Bool(false),
		},
		{
			Name:   "= same bools",
			Bass:   "(= false false)",
			Result: bass.Bool(true),
		},
		{
			Name:   "= different bools",
			Bass:   "(= false true)",
			Result: bass.Bool(false),
		},
		{
			Name:   "= same ints",
			Bass:   "(= 1 1 1)",
			Result: bass.Bool(true),
		},
		{
			Name:   "= different ints",
			Bass:   "(= 1 2 1)",
			Result: bass.Bool(false),
		},
		{
			Name:   "= same strings",
			Bass:   `(= "abc" "abc" "abc")`,
			Result: bass.Bool(true),
		},
		{
			Name:   "= different strings",
			Bass:   `(= "abc" "abc" "def")`,
			Result: bass.Bool(false),
		},
		{
			Name:   "= same symbols",
			Bass:   `(= (quote abc) (quote abc))`,
			Result: bass.Bool(true),
		},
		{
			Name:   "= different symbols",
			Bass:   `(= (quote abc) (quote def))`,
			Result: bass.Bool(false),
		},
		{
			Name:   "= same envs",
			Bass:   "(= (get-current-env) (get-current-env))",
			Result: bass.Bool(true),
		},
		{
			Name:   "= different envs",
			Bass:   "(= (make-env) (make-env))",
			Result: bass.Bool(false),
		},
		{
			Name:   "> decreasing",
			Bass:   "(> 3 2 1)",
			Result: bass.Bool(true),
		},
		{
			Name:   "> decreasing-eq",
			Bass:   "(> 3 2 2)",
			Result: bass.Bool(false),
		},
		{
			Name:   "> increasing",
			Bass:   "(> 1 2 3)",
			Result: bass.Bool(false),
		},
		{
			Name:   "> increasing-eq",
			Bass:   "(> 1 2 2)",
			Result: bass.Bool(false),
		},
		{
			Name:   ">= decreasing",
			Bass:   "(>= 3 2 1)",
			Result: bass.Bool(true),
		},
		{
			Name:   ">= decreasing-eq",
			Bass:   "(>= 3 2 2)",
			Result: bass.Bool(true),
		},
		{
			Name:   ">= increasing",
			Bass:   "(>= 1 2 3)",
			Result: bass.Bool(false),
		},
		{
			Name:   ">= increasing-eq",
			Bass:   "(>= 1 2 2)",
			Result: bass.Bool(false),
		},
		{
			Name:   "< decreasing",
			Bass:   "(< 3 2 1)",
			Result: bass.Bool(false),
		},
		{
			Name:   "< decreasing-eq",
			Bass:   "(< 3 2 2)",
			Result: bass.Bool(false),
		},
		{
			Name:   "< increasing",
			Bass:   "(< 1 2 3)",
			Result: bass.Bool(true),
		},
		{
			Name:   "< increasing-eq",
			Bass:   "(< 1 2 2)",
			Result: bass.Bool(false),
		},
		{
			Name:   "<= decreasing",
			Bass:   "(<= 3 2 1)",
			Result: bass.Bool(false),
		},
		{
			Name:   "<= decreasing-eq",
			Bass:   "(<= 3 2 2)",
			Result: bass.Bool(false),
		},
		{
			Name:   "<= increasing",
			Bass:   "(<= 1 2 3)",
			Result: bass.Bool(true),
		},
		{
			Name:   "<= increasing-eq",
			Bass:   "(<= 1 2 2)",
			Result: bass.Bool(true),
		},
	} {
		test.Run(t)
	}
}

func TestGroundConstructors(t *testing.T) {
	env := bass.NewStandardEnv()

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
			Name:   "quote",
			Bass:   "(quote abc)",
			Result: bass.Symbol("abc"),
		},
		{
			Name: "op",
			Bass: "((op (x) e (cons x e)) y)",
			Result: bass.Pair{
				A: bass.Symbol("y"),
				D: env,
			},
		},
		{
			Name: "bracket op",
			Bass: "((op [x] e (cons x e)) y)",
			Result: bass.Pair{
				A: bass.Symbol("y"),
				D: env,
			},
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
		{
			Name: "assoc",
			Bass: "(assoc {:a 1} :b 2 :c 3)",
			Result: bass.Object{
				"a": bass.Int(1),
				"b": bass.Int(2),
				"c": bass.Int(3),
			},
		},
		{
			Name: "assoc clones",
			Bass: "(def foo {:a 1}) (assoc foo :b 2 :c 3) foo",
			Result: bass.Object{
				"a": bass.Int(1),
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			reader := bytes.NewBufferString(test.Bass)

			res, err := bass.EvalReader(env, reader)
			if test.Err != nil {
				require.ErrorIs(t, err, test.Err)
			} else if test.ErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.ErrContains)
			} else {
				require.NoError(t, err)
				Equal(t, res, test.Result)
			}
		})
	}
}

func TestGroundEnv(t *testing.T) {
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
						A: bass.Cons{
							A: bass.Symbol("c"),
							D: bass.Cons{
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
		{
			Name: "provide",
			Bass: `(def foo :outer)

						 (provide (capture)
							 (def foo :inner)
							 (defn capture [x]
								 [foo x]))

						 [foo (capture 42)]`,
			Result: bass.NewList(
				bass.Keyword("outer"),
				bass.NewList(
					bass.Keyword("inner"),
					bass.Int(42),
				),
			),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			reader := bytes.NewBufferString(test.Bass)

			env := bass.NewStandardEnv()
			env.Set("sentinel", sentinel)

			res, err := bass.EvalReader(env, reader)
			if test.Err != nil {
				require.ErrorIs(t, err, test.Err)
			} else if test.ErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.ErrContains)
			} else {
				require.NoError(t, err)
				Equal(t, res, test.Result)

				if test.Bindings != nil {
					require.Equal(t, test.Bindings, env.Bindings)
				}
			}
		})
	}

	t.Run("environment creation", func(t *testing.T) {
		env := bass.NewStandardEnv()
		env.Set("sentinel", sentinel)

		res, err := bass.EvalReader(env, bytes.NewBufferString("(get-current-env)"))
		require.NoError(t, err)
		Equal(t, res, env)

		res, err = bass.EvalReader(env, bytes.NewBufferString("(make-env)"))
		require.NoError(t, err)

		var created *bass.Env
		err = res.Decode(&created)
		require.NoError(t, err)
		require.Empty(t, created.Bindings)
		require.Empty(t, created.Parents)

		env.Set("created", created)

		res, err = bass.EvalReader(env, bytes.NewBufferString("(make-env (get-current-env) created)"))
		require.NoError(t, err)

		var child *bass.Env
		err = res.Decode(&child)
		require.NoError(t, err)
		require.Empty(t, child.Bindings)
		require.Equal(t, child.Parents, []*bass.Env{env, created})
	})
}

func TestGroundEnvDoc(t *testing.T) {
	reader := bytes.NewBufferString(`
; commentary for environment
; split along multiple lines
_

; a separate comment
;
; with multiple paragraphs
_

; docs for abc
(def abc 123)

; more commentary between abc and quote
_

(defop quote (x) _ x) ; docs for quote

; docs for inc
(defn inc (x) (+ x 1))

(provide [inner]
	; documented inside
	(defn inner [] true))

(comment
	(def commented 123)
	"comments for commented")

(doc abc quote inc inner commented)

(commentary commented)
`)

	env := bass.NewStandardEnv()

	res, err := bass.EvalReader(env, reader)
	require.NoError(t, err)
	require.Equal(t, bass.String("comments for commented"), res)

	require.Contains(t, docsOut.String(), "docs for abc")
	require.Contains(t, docsOut.String(), "number?")
	require.Contains(t, docsOut.String(), "docs for quote")
	require.Contains(t, docsOut.String(), "operative?")
	require.Contains(t, docsOut.String(), "docs for inc")
	require.Contains(t, docsOut.String(), "applicative?")

	docsOut.Reset()

	reader = bytes.NewBufferString(`(doc)`)
	_, err = bass.EvalReader(env, reader)
	require.NoError(t, err)

	require.Contains(t, docsOut.String(), `--------------------------------------------------
commentary for environment split along multiple lines
`)

	require.Contains(t, docsOut.String(), `--------------------------------------------------
abc number?

docs for abc
`)

	require.Contains(t, docsOut.String(), `--------------------------------------------------
a separate comment

with multiple paragraphs
`)

	require.Contains(t, docsOut.String(), `--------------------------------------------------
quote operative? combiner?
args: (x)

docs for quote
`)

	require.Contains(t, docsOut.String(), `--------------------------------------------------
inc applicative? combiner?
args: (x)

docs for inc

`)

	require.Contains(t, docsOut.String(), `--------------------------------------------------
inner applicative? combiner?
args: ()

documented inside

`)

	require.Contains(t, docsOut.String(), `--------------------------------------------------
commented number?

comments for commented

`)
}

func TestGroundBoolean(t *testing.T) {
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
		{
			Name:   "or",
			Bass:   `(or 42 unevaluated)`,
			Result: bass.Int(42),
		},
		{
			Name:   "or false",
			Bass:   `(or false 42)`,
			Result: bass.Int(42),
		},
		{
			Name:   "or false extended",
			Bass:   `(or false null "yep")`,
			Result: bass.String("yep"),
		},
		{
			Name:   "or last null",
			Bass:   `(or false null)`,
			Result: bass.Null{},
		},
		{
			Name:   "or last false",
			Bass:   `(or false false)`,
			Result: bass.Bool(false),
		},
		{
			Name:   "or empty",
			Bass:   `(or)`,
			Result: bass.Bool(false),
		},
		{
			Name:   "and false",
			Bass:   `(and false unevaluated)`,
			Result: bass.Bool(false),
		},
		{
			Name:   "and true",
			Bass:   `(and true 42)`,
			Result: bass.Int(42),
		},
		{
			Name:   "and true extended",
			Bass:   `(and true 42 "hello")`,
			Result: bass.String("hello"),
		},
		{
			Name:   "and empty",
			Bass:   `(and)`,
			Result: bass.Bool(true),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			reader := bytes.NewBufferString(test.Bass)

			env := bass.NewStandardEnv()
			env.Set("sentinel", sentinel)

			res, err := bass.EvalReader(env, reader)
			if test.Err != nil {
				require.ErrorIs(t, err, test.Err)
			} else if test.ErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.ErrContains)
			} else {
				require.NoError(t, err)
				Equal(t, res, test.Result)

				if test.Bindings != nil {
					require.Equal(t, test.Bindings, env.Bindings)
				}
			}
		})
	}
}

func TestGroundStdlib(t *testing.T) {
	type example struct {
		Name string
		Bass string

		Result   bass.Value
		Bindings bass.Bindings

		Err         error
		ErrContains string
	}

	for _, test := range []example{
		{
			Name:   "do",
			Bass:   "(do (def a 1) (def b 2) [a b])",
			Result: bass.NewList(bass.Int(1), bass.Int(2)),
			Bindings: bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
			},
		},
		{
			Name:   "list",
			Bass:   "(list (def a 42) a)",
			Result: bass.NewList(bass.Symbol("a"), bass.Int(42)),
			Bindings: bass.Bindings{
				"a": bass.Int(42),
			},
		},
		{
			Name: "list*",
			Bass: "(list* (def a 1) a (list (def b 2) b))",
			Result: bass.NewList(
				bass.Symbol("a"),
				bass.Int(1),
				bass.Symbol("b"),
				bass.Int(2),
			),
			Bindings: bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
			},
		},
		{
			Name:   "first",
			Bass:   "(first (list 1 2 3))",
			Result: bass.Int(1),
		},
		{
			Name:   "rest",
			Bass:   "(rest (list 1 2 3))",
			Result: bass.NewList(bass.Int(2), bass.Int(3)),
		},
		{
			Name:   "second",
			Bass:   "(second (list 1 2 3))",
			Result: bass.Int(2),
		},
		{
			Name:   "third",
			Bass:   "(third (list 1 2 3))",
			Result: bass.Int(3),
		},
		{
			Name:   "length",
			Bass:   "(length (list 1 2 3))",
			Result: bass.Int(3),
		},
		{
			Name:   "op with multiple exprs",
			Bass:   "((op [x y] e (eval [def x y] e) y) foo 42)",
			Result: bass.Int(42),
			Bindings: bass.Bindings{
				"foo": bass.Int(42),
			},
		},
		{
			Name:   "defop",
			Bass:   `(defop def2 [x y] e (eval [def x y] e) y)`,
			Result: bass.Symbol("def2"),
		},
		{
			Name:   "defop call",
			Bass:   `(defop def2 [x y] e (eval [def x y] e) y) (def2 foo 42)`,
			Result: bass.Int(42),
		},
		{
			Name:     "fn",
			Bass:     "((fn [x] (def local (* x 2)) [local (* local 2)]) 21)",
			Result:   bass.NewList(bass.Int(42), bass.Int(84)),
			Bindings: bass.Bindings{},
		},
		{
			Name:   "defn",
			Bass:   "(defn foo [x] (def local (* x 2)) [local (* local 2)])",
			Result: bass.Symbol("foo"),
		},
		{
			Name:   "defn call",
			Bass:   "(defn foo [x] (def local (* x 2)) [local (* local 2)]) (foo 21)",
			Result: bass.NewList(bass.Int(42), bass.Int(84)),
		},
		{
			Name: "map",
			Bass: "(map (fn [x] (* x 2)) [1 2 3])",
			Result: bass.NewList(
				bass.Int(2),
				bass.Int(4),
				bass.Int(6),
			),
		},
		{
			Name:   "cond first",
			Bass:   "(cond true 1 unevaluated unevaluated)",
			Result: bass.Int(1),
		},
		{
			Name:   "cond second",
			Bass:   "(cond false unevaluated true 2 unevaluated unevaluated)",
			Result: bass.Int(2),
		},
		{
			Name:   "cond else",
			Bass:   "(cond false unevaluated false unevaluated :else 3)",
			Result: bass.Int(3),
		},
		{
			Name:   "cond none",
			Bass:   "(cond false unevaluated false unevaluated false unevaluated)",
			Result: bass.Null{},
		},
		{
			Name:   "let",
			Bass:   "(let [a 21 b (* a 2)] [a b])",
			Result: bass.NewList(bass.Int(21), bass.Int(42)),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			env := bass.NewStandardEnv()

			res, err := bass.EvalReader(env, bytes.NewBufferString(test.Bass))
			if test.Err != nil {
				require.ErrorIs(t, err, test.Err)
			} else if test.ErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.ErrContains)
			} else {
				require.NoError(t, err)
				Equal(t, res, test.Result)

				if test.Bindings != nil {
					require.Equal(t, test.Bindings, env.Bindings)
				}
			}
		})
	}
}

func TestGroundPipes(t *testing.T) {
	env := bass.NewStandardEnv()

	type example struct {
		Name string
		Bass string

		Stdin  []bass.Value
		Err    error
		Result bass.Value
		Stdout []bass.Value
	}

	for _, test := range []example{
		{
			Name:   "*stdin*",
			Bass:   "*stdin*",
			Result: bass.Stdin,
		},
		{
			Name:   "*stdout*",
			Bass:   "*stdout*",
			Result: bass.Stdout,
		},
		{
			Name:   "emit",
			Bass:   "(emit 42 sink)",
			Result: bass.Null{},
			Stdout: []bass.Value{bass.Int(42)},
		},
		{
			Name:   "next",
			Bass:   "(next source)",
			Stdin:  []bass.Value{bass.Int(42)},
			Result: bass.Int(42),
		},
		{
			Name:  "next end no default",
			Bass:  "(next source)",
			Stdin: []bass.Value{},
			Err:   bass.ErrEndOfSource,
		},
		{
			Name:   "next end with default",
			Bass:   "(next source :default)",
			Stdin:  []bass.Value{},
			Result: bass.Keyword("default"),
		},
		{
			Name:   "last single val",
			Bass:   "(last source)",
			Stdin:  []bass.Value{bass.Int(1)},
			Result: bass.Int(1),
		},
		{
			Name:   "last three vals",
			Bass:   "(last source)",
			Stdin:  []bass.Value{bass.Int(1), bass.Int(2), bass.Int(3)},
			Result: bass.Int(3),
		},
		{
			Name:  "last end no default",
			Bass:  "(last source)",
			Stdin: []bass.Value{},
			Err:   bass.ErrEndOfSource,
		},
		{
			Name:   "last end with default",
			Bass:   "(last source :default)",
			Stdin:  []bass.Value{},
			Result: bass.Keyword("default"),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			sinkBuf := new(bytes.Buffer)

			env.Set("sink", &bass.Sink{bass.NewJSONSink("test", sinkBuf)})

			sourceBuf := new(bytes.Buffer)
			sourceEnc := json.NewEncoder(sourceBuf)
			for _, val := range test.Stdin {
				err := sourceEnc.Encode(val)
				require.NoError(t, err)
			}

			env.Set("source", &bass.Source{bass.NewJSONSource("test", sourceBuf)})

			reader := bytes.NewBufferString(test.Bass)

			res, err := bass.EvalReader(env, reader)
			if test.Err != nil {
				require.ErrorIs(t, err, test.Err)
			} else {
				require.NoError(t, err)
				Equal(t, res, test.Result)
			}

			stdoutSource := bass.NewJSONSource("test", sinkBuf)

			for _, val := range test.Stdout {
				next, err := stdoutSource.Next(context.Background())
				require.NoError(t, err)
				Equal(t, next, val)
			}
		})
	}
}

func TestGroundStrings(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name:   "symbol->string",
			Bass:   "(symbol->string (quote $foo-bar))",
			Result: bass.String("$foo-bar"),
		},
		{
			Name:   "string->symbol",
			Bass:   `(string->symbol "$foo-bar")`,
			Result: bass.Symbol("$foo-bar"),
		},
		{
			Name:   "str",
			Bass:   `(str "foo" (quote bar) "baz" _ :buzz)`,
			Result: bass.String("foobarbaz_:buzz"),
		},
		{
			Name:   "substring offset",
			Bass:   `(substring "abcde" 1)`,
			Result: bass.String("bcde"),
		},
		{
			Name:   "substring range",
			Bass:   `(substring "abcde" 1 3)`,
			Result: bass.String("bc"),
		},
		{
			Name: "substring extra arg",
			Bass: `(substring "abcde" 1 3 5)`,
			Err: bass.ArityError{
				Name: "substring",
				Need: 3,
				Have: 4,
			},
		},
	} {
		t.Run(example.Name, example.Run)
	}
}
