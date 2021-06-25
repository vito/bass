package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestIgnoreDecode(t *testing.T) {
	var foo string
	err := bass.Ignore{}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, "")

	foo = "some untouched value"
	err = bass.Ignore{}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, "some untouched value")
}

func TestIgnoreEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Ignore{}

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, val, res)
}
