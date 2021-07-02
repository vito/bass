package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestOperativeDecode(t *testing.T) {
	val := operative

	var c bass.Combiner
	err := val.Decode(&c)
	require.NoError(t, err)
	require.Equal(t, val, c)

	var o *bass.Operative
	err = val.Decode(&o)
	require.NoError(t, err)
	require.Equal(t, val, o)
}

func TestOperativeEqual(t *testing.T) {
	val := operative
	require.True(t, val.Equal(val))

	other := *operative
	require.False(t, val.Equal(&other))
}

func TestOperativeCall(t *testing.T) {
	env := bass.NewEnv()
	val := operative

	env.Set("foo", bass.Int(42))

	res, err := Call(val, env, bass.NewList(bass.Symbol("foo")))
	require.NoError(t, err)
	require.Equal(t, bass.Pair{
		A: bass.Symbol("foo"),
		D: env,
	}, res)
}
