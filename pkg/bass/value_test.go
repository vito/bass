package bass_test

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
)

var noopOp = bass.Op("noop", "[]", func() {})
var noopFn = bass.Func("noop", "[]", func() {})

var allConstValues = []bass.Value{
	bass.Null{},
	bass.Ignore{},
	bass.Empty{},
	bass.Bool(true),
	bass.Bool(false),
	bass.Int(42),
	bass.String("hello"),
	noopOp,
	noopFn,
	bass.NewScope(bass.Bindings{
		"a": bass.Symbol("unevaluated"),
		"b": bass.Int(42),
	}),
	operative,
	bass.Wrapped{operative},
	bass.Stdin,
	bass.Stdout,
	bass.DirPath{"dir-path"},
	bass.FilePath{"file-path"},
	bass.CommandPath{"command-path"},
	&bass.Continuation{
		Continue: func(x bass.Value) bass.Value {
			return x
		},
	},
	bass.ThunkPath{
		Thunk: bass.Thunk{
			Args: []bass.Value{
				bass.FilePath{"file"},
			},
		},
		Path: bass.FileOrDirPath{
			Dir: &bass.DirPath{"dir"},
		},
	},
	bass.NewSecret("bruces-secret", []byte("im always angry")),
}

var exprValues = []bass.Value{
	bass.Keyword("major"),
	bass.Symbol("foo"),
	bass.Pair{
		A: bass.Symbol("a"),
		D: bass.Symbol("d"),
	},
	bass.Cons{
		A: bass.Symbol("a"),
		D: bass.Symbol("d"),
	},
	bass.Annotate{
		Value:   bass.Symbol("foo"),
		Comment: "annotated",
		Meta: &bass.Bind{
			bass.Keyword("key"),
			bass.Symbol("foo"),
		},
	},
	bass.Annotated{
		Value: bass.Symbol("foo"),
		Meta: bass.Bindings{
			"key": bass.Symbol("foo"),
		}.Scope(),
	},
	bass.Bind{
		bass.Pair{
			A: bass.Symbol("a"),
			D: bass.Symbol("d"),
		},
	},
	&bass.ReadyContinuation{
		Cont: &bass.Continuation{
			Continue: func(x bass.Value) bass.Value {
				return x
			},
		},
		Result: bass.Int(42),
	},
}

var allValues = append(
	allConstValues,
	exprValues...,
)

func TestConstsDecode(t *testing.T) {
	for _, val := range allValues {
		val := val
		t.Run(val.String(), func(t *testing.T) {
			is := is.New(t)
			var decoded bass.Value
			err := val.Decode(&decoded)
			is.NoErr(err)
			is.Equal(decoded, val)
		})
	}
}

func TestValueOf(t *testing.T) {
	is := is.New(t)

	type example struct {
		src      any
		expected bass.Value
	}

	dummy := &dummyValue{}

	for _, test := range []example{
		{
			dummy,
			dummy,
		},
		{
			nil,
			bass.Null{},
		},
		{
			"foo",
			bass.String("foo"),
		},
		{
			42,
			bass.Int(42),
		},
		{
			json.Number(strconv.Itoa(math.MaxInt64)),
			bass.Int(math.MaxInt64),
		},
		{
			json.Number(fmt.Sprintf("%.5f", math.Pi)),
			bass.String("3.14159"),
		},
		{
			[]string{},
			bass.Empty{},
		},
		{
			[]int{1, 2, 3},
			bass.NewList(
				bass.Int(1),
				bass.Int(2),
				bass.Int(3),
			),
		},
		{
			map[string]any{
				"a": 1,
				"b": true,
				"c": "sup",
			},
			bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Bool(true),
				"c": bass.String("sup"),
			}.Scope(),
		},
		{
			struct {
				A       int    `json:"a"`
				B       bool   `json:"b"`
				C       string `json:"c,omitempty"`
				Ignored int
			}{
				A:       1,
				B:       true,
				C:       "sup",
				Ignored: 42,
			},
			bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Bool(true),
				"c": bass.String("sup"),
			}.Scope(),
		},
		{
			struct {
				A int    `json:"a"`
				B bool   `json:"b"`
				C string `json:"c,omitempty"`
			}{
				A: 1,
				B: true,
			},
			bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Bool(true),
			}.Scope(),
		},
	} {
		actual, err := bass.ValueOf(test.src)
		is.NoErr(err)
		Equal(t, test.expected, actual)
	}
}

func TestString(t *testing.T) {
	type example struct {
		src      bass.Value
		expected string
	}

	dummy := &dummyValue{}

	abcScope := bass.NewEmptyScope()
	for i, k := range []bass.Symbol{"a", "b", "c"} {
		abcScope.Set(k, bass.Int(i+1))
	}

	stableNestedScope := bass.NewEmptyScope(
		bass.Bindings{
			"b": bass.Keyword("parent"),
		}.Scope(bass.Bindings{
			"a": bass.Keyword("grandparent"),
		}.Scope()),
	)
	stableNestedScope.Set("c", bass.Keyword("child"))
	stableNestedScope.Set("a", bass.Keyword("shadowed"))

	for _, test := range []example{
		{
			dummy,
			`<dummy: 0>`,
		},
		{
			bass.Ignore{},
			`_`,
		},
		{
			bass.Null{},
			`null`,
		},
		{
			bass.String("foo"),
			`"foo"`,
		},
		{
			bass.Symbol("foo"),
			`foo`,
		},
		{
			bass.Int(42),
			`42`,
		},
		{
			bass.Empty{},
			`()`,
		},
		{
			bass.NewList(
				bass.Int(1),
				bass.Int(2),
				bass.Int(3),
			),
			`(1 2 3)`,
		},
		{
			bass.NewConsList(
				bass.Int(1),
				bass.Int(2),
				bass.Int(3),
			),
			`[1 2 3]`,
		},
		{
			abcScope,
			`{:a 1 :b 2 :c 3}`,
		},
		{
			bass.Bind{
				bass.Symbol("base"),
				bass.Keyword("a"), bass.Int(1),
				bass.Symbol("b"), bass.Int(2),
				bass.Keyword("c"), bass.Int(3),
			},
			`{base :a 1 b 2 :c 3}`,
		},
		{
			bass.Cons{
				A: bass.Int(1),
				D: bass.Cons{
					A: bass.Int(2),
					D: bass.Int(3),
				},
			},
			`[1 2 & 3]`,
		},
		{
			bass.Pair{
				A: bass.Symbol("foo"),
				D: bass.Symbol("bar"),
			},
			`(foo & bar)`,
		},
		{
			bass.Pair{
				A: bass.Symbol("foo"),
				D: bass.Pair{
					A: bass.Int(2),
					D: bass.Pair{
						A: bass.Int(3),
						D: bass.Empty{},
					},
				},
			},
			`(foo 2 3)`,
		},
		{
			bass.Pair{
				A: bass.Symbol("foo"),
				D: bass.Pair{
					A: bass.Int(2),
					D: bass.Pair{
						A: bass.Int(3),
						D: bass.Symbol("rest"),
					},
				},
			},
			`(foo 2 3 & rest)`,
		},
		{
			bass.Wrapped{
				Underlying: recorderOp{},
			},
			"(wrap <op: recorder>)",
		},
		{
			&bass.Operative{
				Bindings:     bass.Symbol("formals"),
				ScopeBinding: bass.Symbol("eformal"),
				Body:         bass.Symbol("body"),
			},
			"(op formals eformal body)",
		},
		{
			bass.Wrapped{
				Underlying: &bass.Operative{
					Bindings:     bass.Symbol("formals"),
					ScopeBinding: bass.Symbol("eformal"),
					Body:         bass.Symbol("body"),
				},
			},
			"(wrap (op formals eformal body))",
		},
		{
			bass.Wrapped{
				Underlying: &bass.Operative{
					Bindings:     bass.Symbol("formals"),
					ScopeBinding: bass.Ignore{},
					Body:         bass.Symbol("body"),
				},
			},
			"(fn formals body)",
		},
		{
			&bass.Builtin{
				Name:    "banana",
				Formals: bass.Symbol("boat"),
			},
			"<builtin: (banana & boat)>",
		},
		{
			bass.NewEmptyScope(),
			"{}",
		},
		{
			stableNestedScope,
			"{:a :shadowed :b :parent :c :child}",
		},
		{
			bass.Annotate{
				Comment: "hello",
				Value:   bass.Ignore{},
			},
			"_",
		},
		{
			bass.Annotate{
				Value: bass.Ignore{},
				Meta: &bass.Bind{
					bass.Keyword("doc"),
					bass.String("hello"),
				},
			},
			"^{:doc \"hello\"} _",
		},
		{
			bass.Keyword("foo"),
			":foo",
		},
		{
			bass.Keyword("foo_bar"),
			":foo_bar",
		},
		{
			bass.Keyword("foo-bar"),
			":foo-bar",
		},
		{
			bass.Symbol("foo-bar").Unwrap(),
			"(unwrap foo-bar)",
		},
		{
			bass.Stdin,
			"<source: stdin>",
		},
		{
			bass.Stdout,
			"<sink: stdout>",
		},
		{
			bass.DirPath{"foo"},
			"./foo/",
		},
		{
			bass.FilePath{"foo"},
			"./foo",
		},
		{
			bass.CommandPath{"go"},
			".go",
		},
		{
			bass.FilePath{"foo"}.Unwrap(),
			"(unwrap ./foo)",
		},
		{
			bass.CommandPath{"go"}.Unwrap(),
			"(unwrap .go)",
		},
		{
			bass.ExtendPath{
				Parent: bass.DirPath{"foo"},
				Child:  bass.FilePath{"bar"},
			},
			"./foo/bar",
		},
		{
			bass.ExtendPath{
				Parent: bass.DirPath{"foo"},
				Child:  bass.DirPath{"bar"},
			},
			"./foo/bar/",
		},
		{
			bass.ThunkPath{
				Thunk: bass.Thunk{
					Args: []bass.Value{
						bass.FilePath{"file"},
					},
				},
				Path: bass.FileOrDirPath{
					Dir: &bass.DirPath{"dir"},
				},
			},
			"<thunk CBA5NVSCDITAM: (./file)>/dir/",
		},
	} {
		t.Run(fmt.Sprintf("%T", test.src), func(t *testing.T) {
			is := is.New(t)
			is.Equal(test.src.String(), test.expected)
		})
	}
}

func TestResolve(t *testing.T) {
	is := is.New(t)

	res, err := bass.Resolve(
		bass.Bindings{
			"a": bass.Bindings{
				"aa": bass.Int(1),
				"ab": bass.NewList(
					bass.Int(2),
					bass.NewList(
						bass.Int(3),
						bass.Bindings{
							"aba": bass.Int(4),
							"abb": bass.Symbol("abb"),
						}.Scope(),
					),
				),
			}.Scope(),
		}.Scope(),

		func(val bass.Value) (bass.Value, error) {
			var i int
			if err := val.Decode(&i); err == nil {
				return bass.Int(i * 10), nil
			}

			return val, nil
		},
	)
	is.NoErr(err)
	Equal(t, bass.Bindings{
		"a": bass.Bindings{
			"aa": bass.Int(10),
			"ab": bass.NewList(
				bass.Int(20),
				bass.NewList(
					bass.Int(30),
					bass.Bindings{
						"aba": bass.Int(40),
						"abb": bass.Symbol("abb"),
					}.Scope(),
				),
			),
		}.Scope(),
	}.Scope(),

		res)

}
