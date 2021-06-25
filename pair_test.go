package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestPairEval(t *testing.T) {
	env := bass.NewEnv()

	env.Set("foo", bass.String("hello"))
	env.Set("bar", bass.String("world"))

	val := bass.Pair{
		A: bass.Symbol("foo"),
		D: bass.Pair{
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
