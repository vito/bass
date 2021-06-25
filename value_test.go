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
			`[]`,
		},
		{
			bass.NewList(
				bass.Int(1),
				bass.Int(2),
				bass.Int(3),
			),
			`(1 . (2 . (3 . [])))`,
		},
		{
			bass.Apply{
				A: bass.Symbol("foo"),
				D: bass.Pair{
					A: bass.Int(2),
					D: bass.Pair{
						A: bass.Int(3),
						D: bass.Empty{},
					},
				},
			},
			`(foo . (2 . (3 . [])))`,
		},
		{
			bass.Applicative{
				Underlying: recorderOp{},
			},
			"<applicative: <op: recorder>>",
		},
		{
			&bass.Operative{
				Name: "banana",
			},
			"<op: banana>",
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
	} {
		require.Equal(t, test.expected, test.src.String())
	}
}

type dummyValue struct{}

var _ bass.Value = dummyValue{}

func (dummyValue) Decode(interface{}) error               { return nil }
func (dummyValue) String() string                         { return "<dummy>" }
func (val dummyValue) Eval(*bass.Env) (bass.Value, error) { return val, nil }
