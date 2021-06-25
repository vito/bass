package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestSymbolDecode(t *testing.T) {
	var foo string
	err := bass.Symbol("foo").Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, "foo")

	err = bass.Symbol("bar").Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, "bar")
}

func TestSymbolEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Symbol("foo")

	_, err := val.Eval(env)
	require.Equal(t, bass.UnboundError{"foo"}, err)

	env.Set(val, bass.Int(42))

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)
}
