package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestEnvDecode(t *testing.T) {
	env := bass.NewEnv()
	env.Set("foo", bass.Int(42))

	var dest *bass.Env
	err := env.Decode(&dest)
	require.NoError(t, err)
	require.Equal(t, env, dest)
}

func TestEnvEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.NewEnv()
	val.Set("foo", bass.Int(42)) // just to strengthen Equal check

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, val, res)
}

func TestEnvBinding(t *testing.T) {
	env := bass.NewEnv()

	val, found := env.Get("foo")
	require.False(t, found)
	require.Nil(t, val)

	env.Set("foo", bass.Int(42))

	val, found = env.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(42), val)
}

func TestEnvBindingParents(t *testing.T) {
	env := bass.NewEnv()
	env.Set("foo", bass.Int(42))

	child := bass.NewEnv(env)
	val, found := child.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(42), val)
}

func TestEnvBindingParentsOrder(t *testing.T) {
	env1 := bass.NewEnv()
	env1.Set("foo", bass.Int(1))

	env2 := bass.NewEnv()
	env2.Set("foo", bass.Int(2))
	env2.Set("bar", bass.Int(3))

	child := bass.NewEnv(env1, env2)
	val, found := child.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(1), val)

	val, found = child.Get("bar")
	require.True(t, found)
	require.Equal(t, bass.Int(3), val)
}

func TestEnvBindingParentsDepthFirst(t *testing.T) {
	env1Parent := bass.NewEnv()
	env1Parent.Set("foo", bass.Int(1))

	env1 := bass.NewEnv(env1Parent)

	env2 := bass.NewEnv()
	env2.Set("foo", bass.Int(2))

	child := bass.NewEnv(env1, env2)
	val, found := child.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(1), val)
}
