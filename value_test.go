package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

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
			bass.NewInertList(
				bass.Int(1),
				bass.Int(2),
				bass.Int(3),
			),
			`[1 2 3]`,
		},
		{
			bass.InertPair{
				A: bass.Int(1),
				D: bass.InertPair{
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
			bass.Applicative{
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
			bass.Applicative{
				Underlying: &bass.Operative{
					Formals: bass.Symbol("formals"),
					Eformal: bass.Symbol("eformal"),
					Body:    bass.Symbol("body"),
				},
			},
			"(wrap (op formals eformal body))",
		},
		{
			bass.Applicative{
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
	} {
		require.Equal(t, test.expected, test.src.String())
	}
}

type dummyValue struct {
	sentinel int
}

var _ bass.Value = dummyValue{}

func (val dummyValue) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *dummyValue:
		*x = val
		return nil
	default:
		return bass.DecodeError{
			Source:      val,
			Destination: dest,
		}
	}
}

func (val dummyValue) Equal(other bass.Value) bool {
	var o dummyValue
	if err := other.Decode(&o); err != nil {
		return false
	}

	return val.sentinel == o.sentinel
}

func (dummyValue) String() string { return "<dummy>" }

func (val dummyValue) Eval(*bass.Env) (bass.Value, error) {
	return val, nil
}

type wrappedValue struct {
	bass.Value
}
