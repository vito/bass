package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestApplicativeDecode(t *testing.T) {
	val := bass.Applicative{
		Underlying: recorderOp{},
	}

	var a bass.Applicative
	err := val.Decode(&a)
	require.NoError(t, err)
	require.Equal(t, val, a)

	var c bass.Combiner
	err = val.Decode(&c)
	require.NoError(t, err)
	require.Equal(t, val, c)
}

func TestApplicativeEqual(t *testing.T) {
	val := bass.Applicative{
		Underlying: recorderOp{},
	}

	require.True(t, val.Equal(val))
	require.False(t, val.Equal(bass.Func("noop", func() {})))
}

func TestApplicativeCall(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Applicative{
		Underlying: recorderOp{},
	}

	env.Set("foo", bass.Int(42))

	res, err := val.Call(bass.NewList(bass.Symbol("foo")), env)
	require.NoError(t, err)
	require.Equal(t, recorderOp{
		Applied: bass.NewList(bass.Int(42)),
		Env:     env,
	}, res)
}
