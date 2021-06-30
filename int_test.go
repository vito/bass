package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestIntDecode(t *testing.T) {
	var foo int
	err := bass.Int(42).Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, 42)

	err = bass.Int(0).Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, 0)

	var i bass.Int
	err = bass.Int(42).Decode(&i)
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), i)
}

func TestIntEqual(t *testing.T) {
	require.True(t, bass.Int(42).Equal(bass.Int(42)))
	require.True(t, bass.Int(0).Equal(bass.Int(0)))
	require.False(t, bass.Int(42).Equal(bass.Int(0)))
	require.True(t, bass.Int(42).Equal(wrappedValue{bass.Int(42)}))
	require.False(t, bass.Int(42).Equal(wrappedValue{bass.Int(0)}))
}

func TestIntEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Int(42)

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, val, res)
}
