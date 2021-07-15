package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestApplicativeDecode(t *testing.T) {
	val := bass.Wrapped{
		Underlying: recorderOp{},
	}

	var a bass.Wrapped
	err := val.Decode(&a)
	require.NoError(t, err)
	require.Equal(t, val, a)

	var c bass.Combiner
	err = val.Decode(&c)
	require.NoError(t, err)
	require.Equal(t, val, c)
}

func TestApplicativeEqual(t *testing.T) {
	val := bass.Wrapped{
		Underlying: recorderOp{},
	}

	require.True(t, val.Equal(val))
	require.False(t, val.Equal(bass.Func("noop", func() {})))
}

func TestApplicativeCall(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Wrapped{
		Underlying: recorderOp{},
	}

	env.Set("foo", bass.Int(42))

	res, err := Call(val, env, bass.NewList(bass.Symbol("foo")))
	require.NoError(t, err)
	require.Equal(t, recorderOp{
		Applied: bass.NewList(bass.Int(42)),
		Env:     env,
	}, res)
}
