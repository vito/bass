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

func TestEnvEqual(t *testing.T) {
	val := bass.NewEnv()
	require.True(t, val.Equal(val))
	require.False(t, val.Equal(bass.NewEnv()))
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

func TestEnvBindingDocs(t *testing.T) {
	env := bass.NewEnv()

	annotated, found := env.GetWithDoc("foo")
	require.False(t, found)
	require.Zero(t, annotated)
	require.Empty(t, env.Commentary)

	env.Set("foo", bass.Int(42), "hello", "More info.")

	annotated, found = env.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello\n\nMore info.", annotated.Comment)
	require.Equal(t, bass.Int(42), annotated.Value)
	require.NotZero(t, annotated.Range)

	commentary := annotated
	commentary.Value = bass.Symbol("foo")
	require.Equal(t, env.Commentary, []bass.Annotated{commentary})
}

func TestEnvBindingParentsDoc(t *testing.T) {
	env := bass.NewEnv()
	env.Set("foo", bass.Int(42), "hello")

	child := bass.NewEnv(env)
	annotated, found := child.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello", annotated.Comment)
	require.Equal(t, bass.Int(42), annotated.Value)
}

func TestEnvBindingParentsOrderDoc(t *testing.T) {
	env1 := bass.NewEnv()
	env1.Set("foo", bass.Int(1), "hello 1")

	env2 := bass.NewEnv()
	env2.Set("foo", bass.Int(2), "hello 2")
	env2.Set("bar", bass.Int(3))

	child := bass.NewEnv(env1, env2)
	annotated, found := child.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello 1", annotated.Comment)
	require.Equal(t, bass.Int(1), annotated.Value)

	annotated, found = child.GetWithDoc("bar")
	require.True(t, found)
	require.Equal(t, "", annotated.Comment)
	require.Equal(t, bass.Int(3), annotated.Value)
}

func TestEnvBindingParentsDepthFirstDoc(t *testing.T) {
	env1Parent := bass.NewEnv()
	env1Parent.Set("foo", bass.Int(1), "hello 1")

	env1 := bass.NewEnv(env1Parent)

	env2 := bass.NewEnv()
	env2.Set("foo", bass.Int(2), "hello 2")

	child := bass.NewEnv(env1, env2)
	annotated, found := child.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello 1", annotated.Comment)
	require.Equal(t, bass.Int(1), annotated.Value)
}
