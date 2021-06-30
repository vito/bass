package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestNullDecode(t *testing.T) {
	var n bass.Null
	err := bass.Null{}.Decode(&n)
	require.NoError(t, err)
	require.Equal(t, n, bass.Null{})

	var foo string
	err = bass.Null{}.Decode(&foo)
	require.Error(t, err)

	var b bool = true
	err = bass.Null{}.Decode(&b)
	require.NoError(t, err)
	require.False(t, b)
}

func TestNullEqual(t *testing.T) {
	require.True(t, bass.Null{}.Equal(bass.Null{}))
	require.True(t, bass.Null{}.Equal(wrappedValue{bass.Null{}}))
	require.False(t, bass.Null{}.Equal(bass.Bool(false)))
}

func TestNullEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Null{}

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, val, res)
}
