package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestBoolDecode(t *testing.T) {
	var foo bool
	err := bass.Bool(true).Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, true)

	err = bass.Bool(false).Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, false)
}

func TestBoolEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Bool(true)

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, val, res)
}
