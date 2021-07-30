package bass_test

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

var allConstValues = []bass.Value{
	bass.Null{},
	bass.Ignore{},
	bass.Empty{},
	bass.Bool(true),
	bass.Bool(false),
	bass.Int(42),
	bass.Keyword("major"),
	bass.String("hello"),
	bass.Op("noop", func() {}),
	bass.Func("nofn", func() {}),
	bass.NewEnv(),
	bass.Object{
		"a": bass.Symbol("unevaluated"),
		"b": bass.Int(42),
	},
	operative,
	bass.Wrapped{operative},
	bass.Stdin,
	bass.Stdout,
	bass.DirectoryPath{"directory-path"},
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
}

var exprValues = []bass.Value{
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
	bass.Assoc{
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
			var decoded bass.Value
			err := val.Decode(&decoded)
			require.NoError(t, err)
			require.Equal(t, val, decoded)
		})
	}
}

func TestValueOf(t *testing.T) {
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
			bass.Object{
				"a": bass.Int(1),
				"b": bass.Bool(true),
				"c": bass.String("sup"),
			},
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
			bass.Object{
				"a": bass.Int(1),
				"b": bass.Bool(true),
				"c": bass.String("sup"),
			},
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
			bass.Object{
				"a": bass.Int(1),
				"b": bass.Bool(true),
			},
		},
	} {
		actual, err := bass.ValueOf(test.src)
		require.NoError(t, err)
		require.Equal(t, test.expected, actual)
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
			bass.Object{
				bass.Keyword("a"): bass.Int(1),
				bass.Keyword("b"): bass.Int(2),
				bass.Keyword("c"): bass.Int(3),
			},
			`{:a 1 :b 2 :c 3}`,
		},
		{
			bass.Assoc{
				{bass.Keyword("a"), bass.Int(1)},
				{bass.Keyword("b"), bass.Int(2)},
				{bass.Keyword("c"), bass.Int(3)},
			},
			`{:a 1 :b 2 :c 3}`,
		},
		{
			bass.Cons{
				A: bass.Int(1),
				D: bass.Cons{
					A: bass.Int(2),
					D: bass.Int(3),
				},
			},
			`[1 2 . 3]`,
		},
		{
			bass.Pair{
				A: bass.Symbol("foo"),
				D: bass.Symbol("bar"),
			},
			`(foo . bar)`,
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
			`(foo 2 3 . rest)`,
		},
		{
			bass.Wrapped{
				Underlying: recorderOp{},
			},
			"(wrap <op: recorder>)",
		},
		{
			&bass.Operative{
				Formals: bass.Symbol("formals"),
				Eformal: bass.Symbol("eformal"),
				Body:    bass.Symbol("body"),
			},
			"(op formals eformal body)",
		},
		{
			bass.Wrapped{
				Underlying: &bass.Operative{
					Formals: bass.Symbol("formals"),
					Eformal: bass.Symbol("eformal"),
					Body:    bass.Symbol("body"),
				},
			},
			"(wrap (op formals eformal body))",
		},
		{
			bass.Wrapped{
				Underlying: &bass.Operative{
					Formals: bass.Symbol("formals"),
					Eformal: bass.Ignore{},
					Body:    bass.Symbol("body"),
				},
			},
			"(fn formals body)",
		},
		{
			&bass.Builtin{
				Name: "banana",
			},
			"<builtin op: banana>",
		},
		{
			bass.NewEnv(),
			"<env>",
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
			":foo-bar",
		},
		{
			bass.Keyword("foo_bar").Unwrap(),
			"(unwrap :foo-bar)",
		},
		{
			bass.Assoc{
				{bass.Keyword("a"), bass.Int(1)},
				{bass.Symbol("b"), bass.Int(2)},
			},
			"{:a 1 b 2}",
		},
		{
			bass.Object{
				"a": bass.Int(1),
				"b": bass.Int(2),
			},
			"{:a 1 :b 2}",
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
			bass.DirectoryPath{"foo"},
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
				Parent: bass.DirectoryPath{"foo"},
				Child:  bass.FilePath{"bar"},
			},
			"foo/bar",
		},
		{
			bass.ExtendPath{
				Parent: bass.DirectoryPath{"foo"},
				Child:  bass.DirectoryPath{"bar"},
			},
			"foo/bar/",
		},
	} {
		require.Equal(t, test.expected, test.src.String())
	}
}

func TestResolve(t *testing.T) {
	res, err := bass.Resolve(
		bass.Object{
			"a": bass.Object{
				"aa": bass.Int(1),
				"ab": bass.NewList(
					bass.Int(2),
					bass.NewList(
						bass.Int(3),
						bass.Object{
							"aba": bass.Int(4),
							"abb": bass.Symbol("abb"),
						},
					),
				),
			},
		},
		func(val bass.Value) (bass.Value, error) {
			var i int
			if err := val.Decode(&i); err == nil {
				return bass.Int(i * 10), nil
			}

			return val, nil
		},
	)
	require.NoError(t, err)
	Equal(t, bass.Object{
		"a": bass.Object{
			"aa": bass.Int(10),
			"ab": bass.NewList(
				bass.Int(20),
				bass.NewList(
					bass.Int(30),
					bass.Object{
						"aba": bass.Int(40),
						"abb": bass.Symbol("abb"),
					},
				),
			),
		},
	}, res)
}
