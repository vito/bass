package bass_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/spy16/slurp/reader"
	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/zapctx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

var operative = &bass.Operative{
	Bindings:     bass.NewList(bass.Symbol("form")),
	ScopeBinding: bass.Symbol("scope"),
	Body: bass.Cons{
		A: bass.Symbol("form"),
		D: bass.Symbol("scope"),
	},
}

var quoteOp = bass.Op("quote", "[form]", func(scope *bass.Scope, form bass.Value) bass.Value {
	return form
})

var idFn = bass.Func("id", "[val]", func(val bass.Value) bass.Value {
	return val
})

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

var bind = bass.Bind{
	bass.Keyword("a"), bass.Int(1),
	bass.Keyword("b"), bass.Bool(true),
}

var sym = Const{
	Value: bass.Symbol("sym"),
}

type BasicExample struct {
	Name string

	Scope *bass.Scope
	Bind  bass.Bindings
	Bass  string

	Result           bass.Value
	ResultConsistsOf bass.List

	Stderr string
	Log    []string

	Err         error
	ErrEqual    error
	ErrContains string
}

func (example BasicExample) Run(t *testing.T) {
	t.Run(example.Name, func(t *testing.T) {
		scope := example.Scope
		if scope == nil {
			scope = bass.NewStandardScope()
		}

		if example.Bind != nil {
			for k, v := range example.Bind {
				scope.Set(k, v)
			}
		}

		reader := bytes.NewBufferString(example.Bass)

		ctx := context.Background()

		stderrBuf := new(bytes.Buffer)
		ctx = ioctx.StderrToContext(ctx, stderrBuf)

		zapcfg := zap.NewDevelopmentEncoderConfig()
		zapcfg.EncodeTime = nil

		logBuf := new(zaptest.Buffer)
		logger := zap.New(zapcore.NewCore(
			zapcore.NewConsoleEncoder(zapcfg),
			zapcore.AddSync(logBuf),
			zapcore.DebugLevel,
		))

		ctx = zapctx.ToContext(ctx, logger)

		res, err := bass.EvalReader(ctx, scope, reader)

		if example.Err != nil {
			require.ErrorIs(t, err, example.Err)
		} else if example.ErrEqual != nil {
			require.Equal(t, err, example.ErrEqual)
		} else if example.ErrContains != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), example.ErrContains)
		} else if example.ResultConsistsOf != nil {
			require.NoError(t, err)

			expected, err := bass.ToSlice(example.ResultConsistsOf)
			require.NoError(t, err)

			var actualList bass.List
			err = res.Decode(&actualList)
			require.NoError(t, err)

			actual, err := bass.ToSlice(actualList)
			require.NoError(t, err)

			require.ElementsMatch(t, actual, expected)
		} else {
			require.NoError(t, err)
			Equal(t, res, example.Result)
		}

		if example.Stderr != "" {
			require.Equal(t, example.Stderr, stderrBuf.String())
		}

		if example.Log != nil {
			lines := logBuf.Lines()
			require.Len(t, lines, len(example.Log))

			for i, re := range example.Log {
				require.Regexp(t, re, lines[i])
			}
		}
	})
}

func TestGroundPrimitivePredicates(t *testing.T) {
	scope := bass.NewStandardScope()

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
			Name: "empty?",
			Trues: []bass.Value{
				bass.NewEmptyScope(),
				bass.NewScope(bass.Bindings{}, bass.NewEmptyScope()),
				bass.NewEmptyScope(bass.NewEmptyScope()),
				bass.Bind{},
				bass.Null{},
				bass.Empty{},
				bass.String(""),
			},
			Falses: []bass.Value{
				bass.String("a"),
				bass.NewScope(bass.Bindings{"a": bass.Ignore{}}),
				bass.NewScope(bass.Bindings{"a": bass.Ignore{}}, bass.NewEmptyScope()),
				bass.Bool(false),
				bass.Ignore{},
				bind,
			},
		},
		{
			Name: "pair?",
			Trues: []bass.Value{
				pair,
			},
			Falses: []bass.Value{
				bass.Empty{},
				bass.Ignore{},
				bass.Null{},
				scope,
				bind,
				cons,
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
				scope,
				bind,
			},
		},
		{
			Name: "scope?",
			Trues: []bass.Value{
				scope,
			},
			Falses: []bass.Value{
				bass.Empty{},
				bass.Ignore{},
				bass.Null{},
				bind,
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
				quoteOp,
				bass.Symbol("sup"),
				bass.CommandPath{"foo"},
				bass.FilePath{"foo"},
			},
			Falses: []bass.Value{
				bass.Keyword("sup"),
				bass.DirPath{"foo"},
			},
		},
		{
			Name: "applicative?",
			Trues: []bass.Value{
				idFn,
				bass.Symbol("sup"),
				bass.CommandPath{"foo"},
				bass.FilePath{"foo"},
			},
			Falses: []bass.Value{
				bass.Keyword("sup"),
				quoteOp,
				bass.DirPath{"foo"},
			},
		},
		{
			Name: "operative?",
			Trues: []bass.Value{
				quoteOp,
				operative,
			},
			Falses: []bass.Value{
				idFn,
				bass.Keyword("sup"),
				bass.Symbol("sup"),
				bass.CommandPath{"foo"},
				bass.FilePath{"foo"},
				bass.DirPath{"foo"},
			},
		},
		{
			Name: "path?",
			Trues: []bass.Value{
				bass.DirPath{"foo"},
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
					res, err := Eval(scope, bass.Pair{
						A: bass.Symbol(test.Name),
						D: bass.NewList(Const{arg}),
					})
					require.NoError(t, err)
					require.Equal(t, bass.Bool(true), res)
				})
			}

			for _, arg := range test.Falses {
				t.Run(fmt.Sprintf("%v", arg), func(t *testing.T) {
					res, err := Eval(scope, bass.Pair{
						A: bass.Symbol(test.Name),
						D: bass.NewList(Const{arg}),
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

func TestGroundArrow(t *testing.T) {
	for _, test := range []BasicExample{
		{
			Name:   "-> evaluation",
			Bass:   "(let [x 6 y 7] (-> x (* y)))",
			Result: bass.Int(42),
		},
		{
			Name:   "-> order",
			Bass:   "(-> 6 (- 7))",
			Result: bass.Int(-1),
		},
		{
			Name:   "-> non-list",
			Bass:   "(-> 6 (* 7) str)",
			Result: bass.String("42"),
		},
		{
			Name:   "-> non-list chained",
			Bass:   `(-> 6 (* 7) str (str "!"))`,
			Result: bass.String("42!"),
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
			Name:   "= same scopes",
			Bass:   "(= (get-current-scope) (get-current-scope))",
			Result: bass.Bool(true),
		},
		{
			Name:   "= empty scopes",
			Bass:   "(= {} {})",
			Result: bass.Bool(true),
		},
		{
			Name:   "= different scopes",
			Bass:   "(= {:a 1} {:a 2})",
			Result: bass.Bool(false),
		},
		{
			Name:   "= extra left",
			Bass:   "(= {:a 1 :b 2} {:a 1})",
			Result: bass.Bool(false),
		},
		{
			Name:   "= extra right",
			Bass:   "(= {:a 1} {:a 1 :b 2})",
			Result: bass.Bool(false),
		},
		{
			Name:   "= equal",
			Bass:   "(= {:a 1 :b 2} {:a 1 :b 2})",
			Result: bass.Bool(true),
		},
		{
			Name:   "= different key",
			Bass:   "(= {:a 1 :b 2} {:a 1 :c 2})",
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
	scope := bass.NewStandardScope()

	for _, example := range []BasicExample{
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
			Name:  "op",
			Scope: scope,
			Bass:  "((op (x) e (cons x e)) y)",
			Result: bass.Pair{
				A: bass.Symbol("y"),
				D: scope,
			},
		},
		{
			Name:  "bracket op",
			Scope: scope,
			Bass:  "((op [x] e (cons x e)) y)",
			Result: bass.Pair{
				A: bass.Symbol("y"),
				D: scope,
			},
		},
		{
			Name:   "wrap",
			Bass:   "((wrap (op x _ x)) 1 2 (+ 1 2))",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			Name: "unwrap",
			Bind: bass.Bindings{
				"operative": operative,
			},
			Bass:   "(unwrap (wrap operative))",
			Result: operative,
		},
		{
			Name: "assoc",
			Bass: "(assoc {:a 1} :b 2 :c 3)",
			Result: bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
				"c": bass.Int(3),
			}.Scope(),
		},
		{
			Name: "assoc clones",
			Bass: "(def foo {:a 1}) (assoc foo :b 2 :c 3) foo",
			Result: bass.Bindings{
				"a": bass.Int(1)}.Scope(),
		},
		{
			Name:     "error",
			Bass:     `(error "oh no!")`,
			ErrEqual: errors.New("oh no!"),
		},
		{
			Name:     "errorf",
			Bass:     `(errorf "oh no! %s: %d" "bam" 42)`,
			ErrEqual: fmt.Errorf("oh no! bam: 42"),
		},
		{
			Name:   "now minute",
			Bass:   `(now 60)`,
			Result: bass.String(fakeClock.Now().Truncate(time.Minute).UTC().Format(time.RFC3339)),
		},
		{
			Name:   "now hour",
			Bass:   `(now 3600)`,
			Result: bass.String(fakeClock.Now().Truncate(time.Hour).UTC().Format(time.RFC3339)),
		},
	} {
		t.Run(example.Name, example.Run)
	}
}

func TestGroundScope(t *testing.T) {
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
			Bass: "(def (a & bs) [1 2 3])",
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
			Bass: "(def (a b [c d] e & fs) [1 2 [3 4] 5 6 7])",
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
			Name:   "bind",
			Bass:   `(let [e (make-scope) b (quote a)] [(bind e b 1) (eval b e)])`,
			Result: bass.NewList(bass.Bool(true), bass.Int(1)),
		},
		{
			Name:   "bind",
			Bass:   `(let [e (make-scope) b 2] [(bind e b 1) (eval b e)])`,
			Result: bass.NewList(bass.Bool(false), bass.Int(2)),
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
				bass.Symbol("outer"),
				bass.NewList(
					bass.Symbol("inner"),
					bass.Int(42),
				),
			),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			reader := bytes.NewBufferString(test.Bass)

			scope := bass.NewStandardScope()
			scope.Set("sentinel", sentinel)

			res, err := bass.EvalReader(context.Background(), scope, reader)
			if test.Err != nil {
				require.ErrorIs(t, err, test.Err)
			} else if test.ErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.ErrContains)
			} else {
				require.NoError(t, err)
				Equal(t, res, test.Result)

				if test.Bindings != nil {
					require.Equal(t, test.Bindings, scope.Bindings)
				}
			}
		})
	}

	t.Run("scope creation", func(t *testing.T) {
		scope := bass.NewStandardScope()
		scope.Set("sentinel", sentinel)

		res, err := bass.EvalReader(context.Background(), scope, bytes.NewBufferString("(get-current-scope)"))
		require.NoError(t, err)
		Equal(t, res, scope)

		res, err = bass.EvalReader(context.Background(), scope, bytes.NewBufferString("(make-scope)"))
		require.NoError(t, err)

		var created *bass.Scope
		err = res.Decode(&created)
		require.NoError(t, err)
		require.Empty(t, created.Bindings)
		require.Empty(t, created.Parents)

		scope.Set("created", created)

		res, err = bass.EvalReader(context.Background(), scope, bytes.NewBufferString("(make-scope (get-current-scope) created)"))
		require.NoError(t, err)

		var child *bass.Scope
		err = res.Decode(&child)
		require.NoError(t, err)
		require.Empty(t, child.Bindings)
		require.Equal(t, child.Parents, []*bass.Scope{scope, created})
	})
}

func TestGroundScopeDoc(t *testing.T) {
	r := bytes.NewBufferString(`
; commentary for scope
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
	commented ; comments for commented
)

(doc abc quote inc inner commented id)

(commentary commented)
`)

	scope := bass.NewStandardScope()

	ctx := context.Background()

	docsOut := new(bytes.Buffer)
	ctx = ioctx.StderrToContext(ctx, colorable.NewNonColorable(docsOut))

	scope.Set("id",
		bass.Func("id", "[val]", func(v bass.Value) bass.Value { return v }),
		"returns val")

	res, err := bass.EvalReader(ctx, scope, r, "(test)")
	require.NoError(t, err)
	require.Equal(t, bass.Annotated{
		Comment: "comments for commented",
		Range: bass.Range{ // XXX: have to keep this up to date
			Start: reader.Position{
				File: "(test)",
				Ln:   28,
				Col:  1,
			},
			End: reader.Position{
				File: "(test)",
				Ln:   28,
				Col:  10,
			},
		},
		Value: bass.AnnotatedBinding{
			Bindable: bass.Symbol("commented"),
			Range: bass.Range{
				Start: reader.Position{
					File: "(test)",
					Ln:   27,
					Col:  6,
				},
				End: reader.Position{
					File: "(test)",
					Ln:   27,
					Col:  15,
				},
			},
		},
	}, res)

	require.Contains(t, docsOut.String(), "docs for abc")
	require.Contains(t, docsOut.String(), "number?")
	require.Contains(t, docsOut.String(), "docs for quote")
	require.Contains(t, docsOut.String(), "operative?")
	require.Contains(t, docsOut.String(), "docs for inc")
	require.Contains(t, docsOut.String(), "applicative?")

	docsOut.Reset()

	r = bytes.NewBufferString(`(doc)`)
	_, err = bass.EvalReader(ctx, scope, r)
	require.NoError(t, err)

	require.Contains(t, docsOut.String(), `--------------------------------------------------
commentary for scope split along multiple lines
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
id applicative? combiner?
args: [val]

returns val
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

	// used as a test value, bound to 'sentinel' to demonstrate that evaluation
	// occurs
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
			Bass:   `(or sentinel unevaluated)`,
			Result: sentinel,
		},
		{
			Name:   "or false",
			Bass:   `(or false sentinel)`,
			Result: sentinel,
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
			Name:   "and true",
			Bass:   `(and true sentinel)`,
			Result: sentinel,
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

			scope := bass.NewStandardScope()
			scope.Set("sentinel", sentinel)

			res, err := bass.EvalReader(context.Background(), scope, reader)
			if test.Err != nil {
				require.ErrorIs(t, err, test.Err)
			} else if test.ErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.ErrContains)
			} else {
				require.NoError(t, err)
				Equal(t, res, test.Result)

				if test.Bindings != nil {
					require.Equal(t, test.Bindings, scope.Bindings)
				}
			}
		})
	}
}

func TestGroundInvariants(t *testing.T) {
	for _, expr := range []string{
		`(= (not x) (if x false true))`,
	} {
		t.Run(expr, func(t *testing.T) {
			for _, val := range allValues {
				t.Run(fmt.Sprintf("%T", val), func(t *testing.T) {
					reader := bytes.NewBufferString(expr)

					scope := bass.NewStandardScope()
					scope.Set("x", val)

					res, err := bass.EvalReader(context.Background(), scope, reader)
					require.NoError(t, err)
					require.Equal(t, bass.Bool(true), res)
				})
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
		{
			Name: "apply",
			Bass: "(apply (fn xs xs) [(quote foo) 42 {:a 1}])",
			Result: bass.NewList(
				bass.Symbol("foo"),
				bass.Int(42),
				bass.Bindings{"a": bass.Int(1)}.Scope(),
			),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			scope := bass.NewStandardScope()

			res, err := bass.EvalReader(context.Background(), scope, bytes.NewBufferString(test.Bass))
			if test.Err != nil {
				require.ErrorIs(t, err, test.Err)
			} else if test.ErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.ErrContains)
			} else {
				require.NoError(t, err)
				Equal(t, res, test.Result)

				if test.Bindings != nil {
					require.Equal(t, test.Bindings, scope.Bindings)
				}
			}
		})
	}
}

func TestGroundPipes(t *testing.T) {
	scope := bass.NewStandardScope()

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
			Name:   "static stream",
			Bass:   "(let [s (stream 1 2 3)] [(next s) (next s) (next s) (next s :end)])",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3), bass.Symbol("end")),
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
			Result: bass.Symbol("default"),
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
			Result: bass.Symbol("default"),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			sinkBuf := new(bytes.Buffer)

			scope.Set("sink", &bass.Sink{bass.NewJSONSink("test", sinkBuf)})

			sourceBuf := new(bytes.Buffer)
			sourceEnc := json.NewEncoder(sourceBuf)
			for _, val := range test.Stdin {
				err := sourceEnc.Encode(val)
				require.NoError(t, err)
			}

			scope.Set("source", &bass.Source{bass.NewJSONSource("test", sourceBuf)})

			reader := bytes.NewBufferString(test.Bass)

			res, err := bass.EvalReader(context.Background(), scope, reader)
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

func TestGroundConversions(t *testing.T) {
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
			Name:   "string->run-path",
			Bass:   `(string->run-path "foo")`,
			Result: bass.CommandPath{"foo"},
		},
		{
			Name:   "string->run-path",
			Bass:   `(string->run-path "./file")`,
			Result: bass.FilePath{"file"},
		},
		{
			Name:   "string->run-path",
			Bass:   `(string->run-path "./dir/")`,
			Result: bass.FilePath{"dir"},
		},
		{
			Name:   "string->run-path",
			Bass:   `(string->run-path "foo/bar")`,
			Result: bass.FilePath{"foo/bar"},
		},
		{
			Name:   "string->fs-path",
			Bass:   `(string->fs-path "foo")`,
			Result: bass.FilePath{"foo"},
		},
		{
			Name:   "string->fs-path",
			Bass:   `(string->fs-path "./file")`,
			Result: bass.FilePath{"file"},
		},
		{
			Name:   "string->fs-path",
			Bass:   `(string->fs-path "./dir/")`,
			Result: bass.DirPath{"dir"},
		},
		{
			Name:   "string->fs-path",
			Bass:   `(string->fs-path "foo/bar")`,
			Result: bass.FilePath{"foo/bar"},
		},
		{
			Name: "list->scope",
			Bass: "(list->scope [:a 1 :b 2 :c 3])",
			Result: bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
				"c": bass.Int(3),
			}.Scope(),
		},
		{
			Name: "scope->list",
			Bass: "(scope->list {:a 1 :b 2 :c 3})",
			ResultConsistsOf: bass.NewList(
				bass.Symbol("a"),
				bass.Int(1),
				bass.Symbol("b"),
				bass.Int(2),
				bass.Symbol("c"),
				bass.Int(3),
			),
		},
	} {
		example.Run(t)
	}
}

func TestGroundStrings(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name:   "str",
			Bass:   `(str "foo" :bar "baz" _ (quote :buzz))`,
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

func TestBuiltinCombiners(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name:   "symbol",
			Bass:   `(:foo {:foo 42})`,
			Result: bass.Int(42),
		},
		{
			Name:   "symbol missing",
			Bass:   `(:foo {:bar 42})`,
			Result: bass.Null{},
		},
		{
			Name:   "symbol default",
			Bass:   `(:foo {:foo 42} "hello")`,
			Result: bass.Int(42),
		},
		{
			Name:   "symbol default missing",
			Bass:   `(:foo {:bar 42} "hello")`,
			Result: bass.String("hello"),
		},
		{
			Name:   "symbol applicative",
			Bass:   `(apply :foo [{:bar 42} (quote foo)])`,
			Result: bass.Symbol("foo"),
		},
		{
			Name: "command path",
			Bass: `(.cat "help")`,
			Result: bass.Bindings{
				"path":     bass.CommandPath{"cat"},
				"stdin":    bass.NewList(bass.String("help")),
				"response": bass.Bindings{"stdout": bass.Bool(true)}.Scope(),
			}.Scope(),
		},
		{
			Name: "command path applicative",
			Bass: `(apply .go [(quote foo)])`,
			Result: bass.Bindings{
				"path":     bass.CommandPath{"go"},
				"stdin":    bass.NewList(bass.Symbol("foo")),
				"response": bass.Bindings{"stdout": bass.Bool(true)}.Scope(),
			}.Scope(),
		},
		{
			Name: "file path",
			Bass: `(./foo "help")`,
			Result: bass.Bindings{
				"path":     bass.FilePath{"./foo"},
				"stdin":    bass.NewList(bass.String("help")),
				"response": bass.Bindings{"stdout": bass.Bool(true)}.Scope(),
			}.Scope(),
		},
		{
			Name: "file path applicative",
			Bass: `(apply ./foo [(quote foo)])`,
			Result: bass.Bindings{
				"path":     bass.FilePath{"./foo"},
				"stdin":    bass.NewList(bass.Symbol("foo")),
				"response": bass.Bindings{"stdout": bass.Bool(true)}.Scope(),
			}.Scope(),
		},
	} {
		t.Run(example.Name, example.Run)
	}
}

func TestGroundObject(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name: "reduce-kv",
			Bass: "(reduce-kv (fn [r k v] (cons [k v] r)) [] {:a 1 :b 2 :c 3})",
			ResultConsistsOf: bass.NewList(
				bass.NewList(bass.Symbol("a"), bass.Int(1)),
				bass.NewList(bass.Symbol("b"), bass.Int(2)),
				bass.NewList(bass.Symbol("c"), bass.Int(3)),
			),
		},
	} {
		t.Run(example.Name, example.Run)
	}
}

func TestGroundList(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name: "conj",
			Bass: "(conj [1 2 3] 4 5 6)",
			Result: bass.NewList(
				bass.Int(1),
				bass.Int(2),
				bass.Int(3),
				bass.Int(4),
				bass.Int(5),
				bass.Int(6),
			),
		},
		{
			Name: "append",
			Bass: "(append [1 2] [3 4 5] [6])",
			Result: bass.NewList(
				bass.Int(1),
				bass.Int(2),
				bass.Int(3),
				bass.Int(4),
				bass.Int(5),
				bass.Int(6),
			),
		},
		{
			Name: "filter",
			Bass: "(filter symbol? [1 :two 3 :four 5 :six])",
			Result: bass.NewList(
				bass.Symbol("two"),
				bass.Symbol("four"),
				bass.Symbol("six"),
			),
		},
		{
			Name: "foldl",
			Bass: "(foldl conj [] [1 2 3])",
			Result: bass.NewList(
				bass.Int(1),
				bass.Int(2),
				bass.Int(3),
			),
		},
		{
			Name: "foldr",
			Bass: "(foldr cons [] [1 2 3])",
			Result: bass.NewList(
				bass.Int(1),
				bass.Int(2),
				bass.Int(3),
			),
		},
	} {
		example.Run(t)
	}
}

func TestGroundDebug(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name:   "dump",
			Bass:   `(dump {:a 1 :b 2})`,
			Result: bass.Bindings{"a": bass.Int(1), "b": bass.Int(2)}.Scope(),
			Stderr: "{\n  \"a\": 1,\n  \"b\": 2\n}\n",
		},
		{
			Name:   "log string",
			Bass:   `(log "hello")`,
			Result: bass.String("hello"),
			Log:    []string{"INFO\thello"},
		},
		{
			Name:   "log non-string",
			Bass:   `(log {:a 1 :b 2})`,
			Result: bass.Bindings{"a": bass.Int(1), "b": bass.Int(2)}.Scope(),
			Log:    []string{"INFO\t{a 1 b 2}"},
		},
		{
			Name:   "logf",
			Bass:   `(logf "oh no! %s: %d" "bam" 42)`,
			Result: bass.Null{},
			Log:    []string{"INFO\toh no! bam: 42"},
		},
		{
			Name:   "time",
			Bass:   `(time (dump 42))`,
			Result: bass.Int(42),
			Log:    []string{`DEBUG\t\(time \(dump 42\)\) => 42 took \d.+s`},
		},
	} {
		t.Run(example.Name, example.Run)
	}
}

func TestGroundCase(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name:   "case matching 1",
			Bass:   "(case 1 1 :one 2 :two _ :more)",
			Result: bass.Symbol("one"),
		},
		{
			Name:   "case matching 2",
			Bass:   "(case 2 1 :one 2 :two _ :more)",
			Result: bass.Symbol("two"),
		},
		{
			Name:   "case matching catch-all",
			Bass:   "(case 3 1 :one 2 :two _ :more)",
			Result: bass.Symbol("more"),
		},
		{
			Name:     "case matching none",
			Bass:     "(case 3 1 :one 2 :two)",
			ErrEqual: fmt.Errorf("no matching case branch: 3"),
		},
		{
			Name:   "case binding",
			Bass:   `(def a 1)[a (case 2 a a) a]`,
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(1)),
		},
		{
			Name:   "case evaluation",
			Bass:   `(case (dump 42) 1 :one 6 :six 42 :forty-two)`,
			Result: bass.Symbol("forty-two"),
			Stderr: "42\n",
		},
	} {
		t.Run(example.Name, example.Run)
	}
}
