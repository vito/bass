package bass_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/mattn/go-colorable"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstest"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/is"
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
	Meta             *bass.Scope
	ResultConsistsOf bass.List
	Binds            bass.Bindings

	Stderr string
	Log    []string

	Err         error
	ErrEqual    error
	ErrContains string
}

func (example BasicExample) Run(t *testing.T) {
	t.Run(example.Name, func(t *testing.T) {
		is := is.New(t)

		scope := example.Scope
		if scope == nil {
			scope = bass.NewStandardScope()
		}

		if example.Bind != nil {
			for k, v := range example.Bind {
				scope.Set(k, v)
			}
		}

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

		reader := bass.NewInMemoryFile(example.Name, example.Bass)
		res, err := bass.EvalFSFile(ctx, scope, reader)

		if example.Err != nil {
			is.True(errors.Is(err, example.Err))
		} else if example.ErrEqual != nil {
			is.Equal(example.ErrEqual.Error(), err.Error())
		} else if example.ErrContains != "" {
			is.True(err != nil)
			is.True(strings.Contains(err.Error(), example.ErrContains))
		} else if example.ResultConsistsOf != nil {
			is.NoErr(err)

			expected, err := bass.ToSlice(example.ResultConsistsOf)
			is.NoErr(err)

			var actualList bass.List
			err = res.Decode(&actualList)
			is.NoErr(err)

			actual, err := bass.ToSlice(actualList)
			is.NoErr(err)

			is.True(cmp.Equal(actual, expected, cmpopts.SortSlices(func(a, b bass.Value) bool {
				return a.String() < b.String()
			})))
		} else {
			is.NoErr(err)

			if example.Result != nil {
				if example.Meta != nil {
					var ann bass.Annotated
					err := res.Decode(&ann)
					is.NoErr(err)

					if !example.Meta.IsSubsetOf(ann.Meta) {
						t.Errorf("meta: %s ⊄ %s\n%s", example.Meta, ann.Meta, cmp.Diff(example.Meta, ann.Meta))
					}
				}

				basstest.Equal(t, example.Result, res)
			} else if example.Binds != nil {
				is.Equal(example.Binds, scope.Bindings)
			}
		}

		if example.Stderr != "" {
			is.Equal(stderrBuf.String(), example.Stderr)
		}

		if example.Log != nil {
			lines := logBuf.Lines()
			is.True(len(lines) == len(example.Log))

			for i, l := range example.Log {
				logRe, err := regexp.Compile(l)
				is.NoErr(err)
				if !logRe.MatchString(lines[i]) {
					t.Errorf("%q does not match %q", lines[i], logRe)
				}
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
				cons,
			},
			Falses: []bass.Value{
				bass.Empty{},
				bass.Ignore{},
				bass.Null{},
				scope,
				bind,
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
				bass.DirPath{"foo"},
			},
			Falses: []bass.Value{
				bass.Keyword("sup"),
			},
		},
		{
			Name: "applicative?",
			Trues: []bass.Value{
				idFn,
				bass.Symbol("sup"),
				bass.CommandPath{"foo"},
				bass.FilePath{"foo"},
				bass.DirPath{"foo"},
			},
			Falses: []bass.Value{
				bass.Keyword("sup"),
				quoteOp,
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
		{
			Name: "thunk?",
			Trues: []bass.Value{
				bass.Thunk{
					Args: []bass.Value{
						bass.FilePath{"foo"},
					},
				},
			},
			Falses: []bass.Value{
				bass.Bindings{
					"cmd": bass.CommandPath{"foo"},
				}.Scope(),
				bass.Bindings{
					"cmd": bass.String("foo"),
				}.Scope(),
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			for _, arg := range test.Trues {
				t.Run(fmt.Sprintf("%v", arg), func(t *testing.T) {
					is := is.New(t)

					res, err := basstest.Eval(scope, bass.Pair{
						A: bass.Symbol(test.Name),
						D: bass.NewList(Const{arg}),
					})
					is.NoErr(err)
					is.Equal(res, bass.Bool(true))
				})
			}

			for _, arg := range test.Falses {
				t.Run(fmt.Sprintf("%v", arg), func(t *testing.T) {
					is := is.New(t)

					res, err := basstest.Eval(scope, bass.Pair{
						A: bass.Symbol(test.Name),
						D: bass.NewList(Const{arg}),
					})
					is.NoErr(err)
					basstest.Equal(t, res, bass.Bool(false))
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
			Name:   "quot",
			Bass:   "(quot 84 2)",
			Result: bass.Int(42),
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
			Name:   "-> return value evaluation",
			Bass:   "(let [x (quote unevaluated) y (fn [_] [x])] (-> x y y))",
			Result: bass.NewList(bass.Symbol("unevaluated")),
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
			Bass:   "(= (current-scope) (current-scope))",
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
			ErrEqual: bass.NewError("oh no!"),
		},
		{
			Name: "error with fields",
			Bass: `(error "oh no!" :a 1 :since {:day 1})`,
			ErrEqual: bass.NewError(
				"oh no!",
				bass.Symbol("a"), bass.Int(1),
				bass.Symbol("since"), bass.Bindings{"day": bass.Int(1)}.Scope(),
			),
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
						A: bass.Pair{
							A: bass.Symbol("c"),
							D: bass.Pair{
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
			Name:   "binds? bound",
			Bass:   `(binds? {:a 1 :b 2} :a)`,
			Result: bass.Bool(true),
		},
		{
			Name:   "binds? not bound",
			Bass:   `(binds? {:a 1 :b 2} :c)`,
			Result: bass.Bool(false),
		},
		{
			Name:   "binds? bound in parent",
			Bass:   `(binds? {:a 1 {:b 2}} :b)`,
			Result: bass.Bool(true),
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
			is := is.New(t)

			scope := bass.NewStandardScope()
			scope.Set("sentinel", sentinel)

			reader := bass.NewInMemoryFile("test", test.Bass)
			res, err := bass.EvalFSFile(context.Background(), scope, reader)
			if test.Err != nil {
				is.True(errors.Is(err, test.Err))
			} else if test.ErrContains != "" {
				is.True(err != nil)
				is.True(strings.Contains(err.Error(), test.ErrContains))
			} else {
				is.NoErr(err)
				basstest.Equal(t, res, test.Result)

				if test.Bindings != nil {
					is.Equal(scope.Bindings, test.Bindings)
				}
			}
		})
	}

	t.Run("scope creation", func(t *testing.T) {
		is := is.New(t)

		scope := bass.NewStandardScope()
		scope.Set("sentinel", sentinel)

		res, err := bass.EvalFSFile(context.Background(), scope, bass.NewInMemoryFile("test", "(current-scope)"))
		is.NoErr(err)
		basstest.Equal(t, res, scope)

		res, err = bass.EvalFSFile(context.Background(), scope, bass.NewInMemoryFile("test", "(make-scope)"))
		is.NoErr(err)

		var created *bass.Scope
		err = res.Decode(&created)
		is.NoErr(err)
		is.True(len(created.Bindings) == 0)
		is.True(len(created.Parents) == 0)

		scope.Set("created", created)

		res, err = bass.EvalFSFile(context.Background(), scope, bass.NewInMemoryFile("test", "(make-scope (current-scope) created)"))
		is.NoErr(err)

		var child *bass.Scope
		err = res.Decode(&child)
		is.NoErr(err)
		is.True(len(child.Bindings) == 0)
		is.Equal([]*bass.Scope{scope, created}, child.Parents)
	})
}

func TestGroundScopeDoc(t *testing.T) {
	is := is.New(t)

	src := `
; commentary split
; along multiple lines
;
; and another paragraph
(def multiline
  _)

; docs for abc
(def abc 123)

(defop quote (x) _ x) ; docs for quote

; docs for inc
(defn inc (x) (+ x 1))

(provide [inner]
  ; documented inside
  (defn inner [] true))

(provide [commented]
  (def some-comment
    "comments for commented")

  ^{:doc some-comment}
  (def commented
    123))

; a schema with embedded docs
(def schema
  {; since day 1
   :a 1

   ; to thine own self
	 :b true})

(doc abc quote inc inner commented schema:a schema:b)

(meta commented)
`

	scope := bass.NewStandardScope()

	ctx := context.Background()

	docsOut := new(bytes.Buffer)
	ctx = ioctx.StderrToContext(ctx, colorable.NewNonColorable(docsOut))

	res, err := bass.EvalFSFile(ctx, scope, bass.NewInMemoryFile("doc test", src))
	is.NoErr(err)
	metaWithoutFile := bass.Bindings{
		"doc":    bass.String("comments for commented"),
		"line":   bass.Int(26),
		"column": bass.Int(2),
	}.Scope()
	is.True(metaWithoutFile.IsSubsetOf(res.(*bass.Scope)))

	t.Log(docsOut.String())

	is.True(strings.Contains(docsOut.String(), "docs for abc"))
	is.True(strings.Contains(docsOut.String(), "number?"))
	is.True(strings.Contains(docsOut.String(), "docs for quote"))
	is.True(strings.Contains(docsOut.String(), "operative?"))
	is.True(strings.Contains(docsOut.String(), "docs for inc"))
	is.True(strings.Contains(docsOut.String(), "applicative?"))
	is.True(strings.Contains(docsOut.String(), "since day 1"))
	is.True(strings.Contains(docsOut.String(), "number?"))
	is.True(strings.Contains(docsOut.String(), "to thine own self"))
	is.True(strings.Contains(docsOut.String(), "boolean?"))

	docsOut.Reset()

	scope.Set("id",
		bass.Func("id", "[val]", func(v bass.Value) bass.Value { return v }),
		"returns val")

	t.Run("commentary", func(t *testing.T) {
		is := is.New(t)

		_, err = bass.EvalFSFile(ctx, scope, bass.NewInMemoryFile("test", "(doc)"))
		is.NoErr(err)

		is.Equal(docsOut.String(), `--------------------------------------------------
multiline ignore?

commentary split along multiple lines

and another paragraph

--------------------------------------------------
abc number?

docs for abc

--------------------------------------------------
quote operative? combiner?
args: (x)

docs for quote

--------------------------------------------------
inc applicative? combiner?
args: (x)

docs for inc

--------------------------------------------------
inner applicative? combiner?
args: ()

documented inside

--------------------------------------------------
commented number?

comments for commented

--------------------------------------------------
schema scope?

a schema with embedded docs

--------------------------------------------------
id applicative? combiner?
args: [val]

returns val

`)
	})
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
		{
			Name:   "when true",
			Bass:   "(when true (def x sentinel) x)",
			Result: sentinel,
			Bindings: bass.Bindings{
				"sentinel": sentinel,
				"x":        sentinel,
			},
		},
		{
			Name:   "when false",
			Bass:   "(when false unevaluated)",
			Result: bass.Null{},
		},
		{
			Name:   "when null",
			Bass:   "(when null unevaluated)",
			Result: bass.Null{},
		},
		{
			Name:   "when empty",
			Bass:   "(when [] sentinel)",
			Result: sentinel,
		},
		{
			Name:   "when string",
			Bass:   `(when "" sentinel)`,
			Result: sentinel,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			is := is.New(t)

			scope := bass.NewStandardScope()
			scope.Set("sentinel", sentinel)

			res, err := bass.EvalFSFile(context.Background(), scope, bass.NewInMemoryFile("test", test.Bass))
			if test.Err != nil {
				is.True(errors.Is(err, test.Err))
			} else if test.ErrContains != "" {
				is.True(err != nil)
				is.True(strings.Contains(err.Error(), test.ErrContains))
			} else {
				is.NoErr(err)
				basstest.Equal(t, res, test.Result)

				if test.Bindings != nil {
					is.Equal(scope.Bindings, test.Bindings)
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
					is := is.New(t)

					scope := bass.NewStandardScope()
					scope.Set("x", val)

					reader := bass.NewInMemoryFile("test", expr)
					res, err := bass.EvalFSFile(context.Background(), scope, reader)
					is.NoErr(err)
					is.Equal(res, bass.Bool(true))
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
			Name:   "curryfn",
			Bass:   "((((curryfn [x y z & more] (* x y z & more)) 2) 3) 4 5 6)",
			Result: bass.Int(720),
		},
		{
			Name:   "curryfn variadic (for some reason)",
			Bass:   "((curryfn more (* & more)) 2 3 4 5 6)",
			Result: bass.Int(720),
		},
		{
			Name:   "curryfn single (for some reason)",
			Bass:   "((curryfn [a] a) 42)",
			Result: bass.Int(42),
		},
		{
			Name:   "curryfn empty (for some reason)",
			Bass:   "((curryfn [] 42))",
			Result: bass.Int(42),
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
			is := is.New(t)

			scope := bass.NewStandardScope()

			reader := bass.NewInMemoryFile("test", test.Bass)
			res, err := bass.EvalFSFile(context.Background(), scope, reader)
			if test.Err != nil {
				is.True(errors.Is(err, test.Err))
			} else if test.ErrContains != "" {
				is.True(err != nil)
				is.True(strings.Contains(err.Error(), test.ErrContains))
			} else {
				is.NoErr(err)
				basstest.Equal(t, res, test.Result)

				if test.Bindings != nil {
					is.Equal(scope.Bindings, test.Bindings)
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
		Sink   []bass.Value
	}

	for _, test := range []example{
		{
			Name:   "static stream",
			Bass:   "(let [s (list->source [1 2 3])] [(next s) (next s) (next s) (next s :end)])",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3), bass.Symbol("end")),
		},
		{
			Name:   "emit",
			Bass:   "(emit 42 sink)",
			Result: bass.Null{},
			Sink:   []bass.Value{bass.Int(42)},
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
		{
			Name:   "take",
			Bass:   "(take 2 (list->source [1 2 3]))",
			Result: bass.NewList(bass.Int(1), bass.Int(2)),
		},
		{
			Name:   "take-all",
			Bass:   "(take-all (list->source [1 2 3]))",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			Name:   "across",
			Bass:   "(next (across (list->source [0 2 4]) (list->source [1 3 5])))",
			Result: bass.NewList(bass.Int(0), bass.Int(1)),
		},
		{
			Name: "for",
			// NB: cheating here a bit by not going over all of them, but it's not
			// worth the complexity as there is nondeterminism here and it's more
			// thoroughly tested in (across) already
			Bass:   "(for [even (list->source [0]) odd (list->source [1])] (emit [even odd] sink))",
			Result: bass.Null{},
			Sink: []bass.Value{
				bass.NewList(bass.Int(0), bass.Int(1)),
			},
		},
		{
			Name:   "collect",
			Bass:   "(collect (fn [x] (+ x 1)) (list->source [1 2 3]))",
			Result: bass.NewList(bass.Int(2), bass.Int(3), bass.Int(4)),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			is := is.New(t)

			sinkBuf := new(bytes.Buffer)

			scope.Set("sink", &bass.Sink{bass.NewJSONSink("test", sinkBuf)})

			sourceBuf := new(bytes.Buffer)
			sourceEnc := json.NewEncoder(sourceBuf)
			for _, val := range test.Stdin {
				err := sourceEnc.Encode(val)
				is.NoErr(err)
			}

			scope.Set("source", &bass.Source{bass.NewJSONSource("test", io.NopCloser(sourceBuf))})

			reader := bass.NewInMemoryFile("test", test.Bass)
			res, err := bass.EvalFSFile(context.Background(), scope, reader)
			if test.Err != nil {
				is.True(errors.Is(err, test.Err))
			} else {
				is.NoErr(err)
				basstest.Equal(t, res, test.Result)
			}

			stdoutSource := bass.NewJSONSource("test", io.NopCloser(sinkBuf))

			for _, val := range test.Sink {
				next, err := stdoutSource.Next(context.Background())
				is.NoErr(err)
				basstest.Equal(t, next, val)
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
			Name:   "string->cmd-path",
			Bass:   `(string->cmd-path "foo")`,
			Result: bass.CommandPath{"foo"},
		},
		{
			Name:   "string->cmd-path",
			Bass:   `(string->cmd-path "./file")`,
			Result: bass.FilePath{"file"},
		},
		{
			Name:   "string->cmd-path",
			Bass:   `(string->cmd-path "./dir/")`,
			Result: bass.FilePath{"dir"},
		},
		{
			Name:   "string->cmd-path",
			Bass:   `(string->cmd-path "foo/bar")`,
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
		{
			Name:   "trim",
			Bass:   "(trim \" \n\tfoo\n\t \")",
			Result: bass.String("foo"),
		},
		{
			Name:   "json",
			Bass:   `(json {:a 1 :b true :multi-word "hello world!\n"})`,
			Result: bass.String(`{"a":1,"b":true,"multi-word":"hello world!\n"}`),
		},
		{
			Name:   "join",
			Bass:   `(use (.strings)) (strings:join ", " ["hello" "world"])`,
			Result: bass.String("hello, world"),
		},
		{
			Name:   "split",
			Bass:   `(use (.strings)) (strings:split "hello, world" ", ")`,
			Result: bass.NewList(bass.String("hello"), bass.String("world")),
		},
		{
			Name:   "upper-case",
			Bass:   `(use (.strings)) (strings:upper-case "hallelujah")`,
			Result: bass.String("HALLELUJAH"),
		},
		{
			Name:   "includes? yes",
			Bass:   `(use (.strings)) (strings:includes? "hello" "he")`,
			Result: bass.Bool(true),
		},
		{
			Name:   "includes? no",
			Bass:   `(use (.strings)) (strings:includes? "hello" "x")`,
			Result: bass.Bool(false),
		},
		{
			Name:   "length",
			Bass:   `(use (.strings)) (strings:length "hello")`,
			Result: bass.Int(5),
		},
	} {
		t.Run(example.Name, example.Run)
	}
}

func TestGroundPaths(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name:   "subpath dir file",
			Bass:   `(subpath ./dir/ ./file)`,
			Result: bass.FilePath{"dir/file"},
		},
		{
			Name:   "subpath dir dir",
			Bass:   `(subpath ./dir/ ./sub/)`,
			Result: bass.DirPath{"dir/sub"},
		},
		{
			Name: "subpath thunk file",
			Bass: `(subpath (.foo) ./sub/)`,
			Result: bass.ThunkPath{
				Thunk: bass.Thunk{
					Args: []bass.Value{
						bass.CommandPath{"foo"},
					},
				},
				Path: bass.FileOrDirPath{
					Dir: &bass.DirPath{"sub"},
				},
			},
		},
		{
			Name: "subpath thunk dir file",
			Bass: `(let [wl (.foo) wl-dir (subpath wl ./dir/)] (subpath wl-dir ./file))`,
			Result: bass.ThunkPath{
				Thunk: bass.Thunk{
					Args: []bass.Value{
						bass.CommandPath{"foo"},
					},
				},
				Path: bass.FileOrDirPath{
					File: &bass.FilePath{"dir/file"},
				},
			},
		},
		{
			Name:   "path-name filepath",
			Bass:   `(path-name ./foo/bar)`,
			Result: bass.String("bar"),
		},
		{
			Name:   "path-name filepath",
			Bass:   `(path-name ./foo/bar.txt)`,
			Result: bass.String("bar.txt"),
		},
		{
			Name:   "path-name dirpath",
			Bass:   `(path-name ./foo/bar/)`,
			Result: bass.String("bar"),
		},
		{
			Name:   "path-name dirpath",
			Bass:   `(path-name ./foo/bar/)`,
			Result: bass.String("bar"),
		},
		{
			Name:   "path-name thunk filepath",
			Bass:   `(path-name (subpath (.foo) ./bar/baz))`,
			Result: bass.String("baz"),
		},
		{
			Name:   "path-name thunk dirpath",
			Bass:   `(path-name (subpath (.foo) ./bar/baz/))`,
			Result: bass.String("baz"),
		},
		{
			Name:   "path-name command",
			Bass:   `(path-name .foo)`,
			Result: bass.String("foo"),
		},
		{
			Name:   "path-stem filepath",
			Bass:   `(path-stem ./foo/bar.baz)`,
			Result: bass.String("bar"),
		},
		{
			Name:   "path-stem dirpath",
			Bass:   `(path-stem ./foo/bar.baz/)`,
			Result: bass.String("bar"),
		},
		{
			Name:   "path-stem dirpath",
			Bass:   `(path-stem ./foo/bar.baz/)`,
			Result: bass.String("bar"),
		},
		{
			Name:   "path-stem thunk filepath",
			Bass:   `(path-stem (subpath (.foo) ./bar/baz.buzz))`,
			Result: bass.String("baz"),
		},
		{
			Name:   "path-stem thunk dirpath",
			Bass:   `(path-stem (subpath (.foo) ./bar/baz.buzz/))`,
			Result: bass.String("baz"),
		},
		{
			Name:   "path-stem command",
			Bass:   `(path-stem .foo)`,
			Result: bass.String("foo"),
		},
	} {
		t.Run(example.Name, example.Run)
	}
}

func TestBuiltinCombiners(t *testing.T) {
	testScope := bass.Bindings{"bar": bass.Int(42)}.Scope()

	for _, example := range []BasicExample{
		{
			Name:   "symbol",
			Bass:   `(:foo {:foo 42})`,
			Result: bass.Int(42),
		},
		{
			Name: "symbol missing",
			Bind: bass.Bindings{"scope": testScope},
			Bass: `(:foo scope)`,
			Err:  bass.UnboundError{"foo", testScope},
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
			Name:   "command path",
			Bass:   `(.cat "meow")`,
			Result: bass.MustThunk(bass.CommandPath{"cat"}, bass.String("meow")),
		},
		{
			Name:   "command path applicative",
			Bass:   `(apply .cat ["meow"])`,
			Result: bass.MustThunk(bass.CommandPath{"cat"}, bass.String("meow")),
		},
		{
			Name:   "file path",
			Bass:   `(./foo "meow")`,
			Result: bass.MustThunk(bass.FilePath{"foo"}, bass.String("meow")),
		},
		{
			Name:   "file path applicative",
			Bass:   `(apply ./foo ["meow"])`,
			Result: bass.MustThunk(bass.FilePath{"foo"}, bass.String("meow")),
		},
		{
			Name: "thunk path",
			Bass: `((subpath (.cat) ./meow) "purr")`,
			Result: bass.MustThunk(
				bass.ThunkPath{
					Thunk: bass.MustThunk(bass.CommandPath{"cat"}),
					Path:  bass.ParseFileOrDirPath("meow"),
				},
				bass.String("purr"),
			),
		},
		{
			Name: "thunk path applicative",
			Bass: `(apply (subpath (.cat) ./meow) ["purr"])`,
			Result: bass.MustThunk(
				bass.ThunkPath{
					Thunk: bass.MustThunk(bass.CommandPath{"cat"}),
					Path:  bass.ParseFileOrDirPath("meow"),
				},
				bass.String("purr"),
			),
		},
	} {
		t.Run(example.Name, example.Run)
	}
}

func TestBassModules(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name: "import",
			Bass: `(import {:a 1 :b 2 :c 3} a b)`,
			Binds: bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
			},
		},
		{
			Name:   "modules",
			Bass:   `((:a (module [a] (def b 6) (def c 7) (defn a [] (* b c)))))`,
			Result: bass.Int(42),
		},
		{
			Name:   "module keys",
			Bass:   `(keys (module [a] (def b 6) (def c 7) (defn a [] (* b c))))`,
			Result: bass.NewList(bass.Symbol("a")),
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
		{
			Name:   "keys",
			Bass:   "(keys {:a 1 :b 2 :c 3})",
			Result: bass.NewList(bass.Symbol("a"), bass.Symbol("b"), bass.Symbol("c")),
		},
		{
			Name:   "vals",
			Bass:   "(vals {:a 1 :b 2 :c 3})",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
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
			Name: "concat",
			Bass: "(concat [1 2] [3 4 5] [6])",
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
			Name:   "log with fields",
			Bass:   `(log "hello" :a 1 :b true)`,
			Result: bass.String("hello"),
			Log:    []string{"INFO\thello\t{\"a\": 1, \"b\": true}"},
		},
		{
			Name:   "log non-string",
			Bass:   `(log {:a 1 :b 2})`,
			Result: bass.Bindings{"a": bass.Int(1), "b": bass.Int(2)}.Scope(),
			Log:    []string{"INFO\t{:a 1 :b 2}"},
		},
		{
			Name:   "log non-string with fields",
			Bass:   `(log {:a 1 :b 2} :to {:thine ["own" "self"]} :b true)`,
			Result: bass.Bindings{"a": bass.Int(1), "b": bass.Int(2)}.Scope(),
			Log:    []string{`INFO\t{:a 1 :b 2}\t\{\"to\": \{\"thine\": \[\"own\", \"self\"\]\}, \"b\": true\}`},
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
			Name: "case matching none",
			Bass: "(case 3 1 :one 2 :two)",
			ErrEqual: &bass.StructuredError{
				Message: "no matching case branch",
				Fields: bass.Bindings{
					"target": bass.Int(3),
				}.Scope(),
			},
		},
		{
			Name:   "case binding",
			Bass:   `(def a 1) [a (case 2 a a) a]`,
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

func TestGroundMeta(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name:   "meta evaluation",
			Bass:   "(def a 1)(meta ^a [])",
			Result: bass.Bindings{"tag": bass.Int(1)}.Scope(),
		},
		{
			// NB: this is consistent with Clojure
			Name:   "meta evaluation is first come first serve",
			Bass:   "(def a 1)(def b 2)(meta ^a ^b [])",
			Result: bass.Bindings{"tag": bass.Int(1)}.Scope(),
		},
		{
			// NB: this is consistent with Clojure
			Name:   "meta evaluation is first come first serve",
			Bass:   "(def a 1)(def b 2)(meta ^b ^a [])",
			Result: bass.Bindings{"tag": bass.Int(2)}.Scope(),
		},
		{
			// NB: this is consistent with Clojure
			Name:   "meta evaluation is first come first serve",
			Bass:   "(def a 1)(def b 2)(meta ^a ^b ^{:x :y :tag 3} [])",
			Result: bass.Bindings{"tag": bass.Int(1), "x": bass.Symbol("y")}.Scope(),
		},
		{
			Name:   "meta reader macro scope",
			Bass:   `(meta ^{:a 1} "since day 1")`,
			Result: bass.Bindings{"a": bass.Int(1)}.Scope(),
		},
		{
			Name:   "meta reader macro symbol",
			Bass:   `(meta ^:b "to thyself")`,
			Result: bass.Bindings{"b": bass.Bool(true)}.Scope(),
		},
		{
			Name: "meta reader macro chain",
			Bass: `(meta ^{:a 1} ^{:b 2} "since week 2")`,
			Result: bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
			}.Scope(),
		},
		{
			Name: "meta reader macro chained with comments",
			Bass: `(meta
			         ^{:a 1} ; comment 1
			         ^{:b 2} ; comment 2
			         "since week 2")`,
			Result: bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
			}.Scope(),
		},
		{
			Name:   "with-meta",
			Bass:   `(with-meta "since day 1" {:a 1})`,
			Result: bass.String("since day 1"),
			Meta:   bass.Bindings{"a": bass.Int(1)}.Scope(),
		},
		{
			Name:   "with-meta existing meta",
			Bass:   `(with-meta (with-meta "to thine own self" {:a 1}) {:b true})`,
			Result: bass.String("to thine own self"),
			Meta:   bass.Bindings{"b": bass.Bool(true)}.Scope(),
		},
		{
			Name: "meta binding",
			Bass: `(def [; im
			             ^:since-day
			             a] [1])
						 (let [{:doc doc
									  :since-day since-day
										:line line
										:column col} (meta a)]
							[doc since-day line col])`,
			// going to great lengths here to avoid doing equality on an *FSPath
			Result: bass.NewList(
				bass.String("im"),
				bass.Bool(true),
				bass.Int(3),
				bass.Int(16),
			),
		},
	} {
		t.Run(example.Name, example.Run)
	}
}
