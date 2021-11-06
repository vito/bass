package bass_test

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
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
	&bass.ReadyContinuation{
		Continuation: &bass.Continuation{
			Continue: func(x bass.Value) bass.Value {
				return x
			},
		},
		Result: bass.Int(42),
	},
	bass.WorkloadPath{
		Workload: bass.Workload{
			Path: bass.RunPath{
				File: &bass.FilePath{"file"},
			},
		},
		Path: bass.FileOrDirPath{
			Dir: &bass.DirPath{"dir"},
		},
	},
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
	bass.Annotated{
		Value:   bass.Symbol("foo"),
		Comment: "annotated",
	},
	bass.Bind{
		bass.Pair{
			A: bass.Symbol("a"),
			D: bass.Symbol("d"),
		},
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
		src      interface{}
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
			map[string]interface{}{
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
		is.Equal(actual, test.expected)
	}
}

func TestString(t *testing.T) {
	type example struct {
		src      bass.Value
		expected string
	}

	dummy := &dummyValue{}

	for _, test := range []example{
		{
			dummy,
			`<dummy>`,
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
			bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
				"c": bass.Int(3),
			}.Scope(),
			`{a 1 b 2 c 3}`,
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
			bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
			}.Scope(),
			"{a 1 b 2}",
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
			"<builtin op: (banana & boat)>",
		},
		{
			bass.NewEmptyScope(),
			"{}",
		},
		{
			bass.NewScope(bass.Bindings{
				"a": bass.Int(42),
				"b": bass.Keyword("hello"),
			}, bass.NewScope(bass.Bindings{
				"c": bass.Int(12),
			}, bass.NewEmptyScope())),
			"{a 42 b :hello {c 12 {}}}",
		},
		{
			bass.Annotated{
				Comment: "hello",
				Value:   bass.Ignore{},
			},
			"_",
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
			"foo/",
		},
		{
			bass.FilePath{"foo"},
			"foo",
		},
		{
			bass.CommandPath{"go"},
			".go",
		},
		{
			bass.FilePath{"foo"}.Unwrap(),
			"(unwrap foo)",
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
			"foo/bar",
		},
		{
			bass.ExtendPath{
				Parent: bass.DirPath{"foo"},
				Child:  bass.DirPath{"bar"},
			},
			"foo/bar/",
		},
		{
			bass.WorkloadPath{
				Workload: bass.Workload{
					Path: bass.RunPath{
						File: &bass.FilePath{"file"},
					},
				},
				Path: bass.FileOrDirPath{
					Dir: &bass.DirPath{"dir"},
				},
			},
			"<workload: a966bb4ef6d955500f26896319657332ae31822a>/dir/",
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
	is.True(bass.Bindings{
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
	}.Scope().Equal(

		res))

}
