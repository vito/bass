package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestApplicativeEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Applicative{
		Underlying: recorderOp{},
	}

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, val, res)
}

func TestApplicativeCall(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Applicative{
		Underlying: recorderOp{},
	}

	env.Set("foo", bass.Int(42))

	res, err := val.Call(bass.Symbol("foo"), env)
	require.NoError(t, err)
	require.Equal(t, recorderOp{
		Applied: bass.Int(42),
		Env:     env,
	}, res)
}
