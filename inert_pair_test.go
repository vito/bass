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

func TestInertPairEqual(t *testing.T) {
	pair := bass.InertPair{
		A: bass.Int(1),
		D: bass.Bool(true),
	}

	wrappedA := bass.InertPair{
		A: wrappedValue{bass.Int(1)},
		D: bass.Bool(true),
	}

	wrappedD := bass.InertPair{
		A: bass.Int(1),
		D: wrappedValue{bass.Bool(true)},
	}

	differentA := bass.InertPair{
		A: bass.Int(2),
		D: bass.Bool(true),
	}

	differentD := bass.InertPair{
		A: bass.Int(1),
		D: bass.Bool(false),
	}

	val := bass.NewEnv()
	require.True(t, pair.Equal(wrappedA))
	require.True(t, pair.Equal(wrappedD))
	require.True(t, wrappedA.Equal(pair))
	require.True(t, wrappedD.Equal(pair))
	require.False(t, pair.Equal(differentA))
	require.False(t, pair.Equal(differentA))
	require.False(t, differentA.Equal(pair))
	require.False(t, differentD.Equal(pair))
	require.False(t, val.Equal(bass.Null{}))

	// not equal to Pair
	require.False(t, pair.Equal(bass.Pair(pair)))
	require.False(t, bass.Pair(pair).Equal(pair))
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
