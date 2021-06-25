package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestInertPairDecode(t *testing.T) {
	pair := bass.InertPair{
		A: bass.Int(1),
		D: bass.Pair{
			A: bass.Bool(true),
			D: bass.String("three"),
		},
	}

	var dest bass.List
	err := pair.Decode(&dest)
	require.NoError(t, err)
	require.Equal(t, pair, dest)
}

func TestInertPairEval(t *testing.T) {
	env := bass.NewEnv()

	env.Set("foo", bass.String("hello"))
	env.Set("bar", bass.String("world"))

	val := bass.InertPair{
		A: bass.Symbol("foo"),
		D: bass.InertPair{
			A: bass.Symbol("bar"),
			D: bass.Empty{},
		},
	}

	expected := bass.Pair{
		A: bass.String("hello"),
		D: bass.Pair{
			A: bass.String("world"),
			D: bass.Empty{},
		},
	}

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, expected, res)
}

func TestInertPairListInterface(t *testing.T) {
	var list bass.List = bass.InertPair{bass.Int(1), bass.Bool(true)}
	require.Equal(t, list.First(), bass.Int(1))
	require.Equal(t, list.Rest(), bass.Bool(true))
}
