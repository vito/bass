package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestAssocDecode(t *testing.T) {
	list := bass.Assoc{
		{bass.Symbol("a"), bass.Int(1)},
		{bass.Symbol("b"), bass.Bool(true)},
		{bass.Symbol("c"), bass.String("three")},
	}

	var obj bass.Assoc
	err := list.Decode(&obj)
	require.NoError(t, err)
	require.Equal(t, list, obj)
}

func TestAssocEqual(t *testing.T) {
	obj := bass.Assoc{
		{bass.Symbol("a"), bass.Int(1)},
		{bass.Symbol("b"), bass.Bool(true)},
	}

	reverse := bass.Assoc{
		{bass.Symbol("a"), bass.Int(1)},
		{bass.Symbol("b"), bass.Bool(true)},
	}

	wrappedVA := bass.Assoc{
		{bass.Symbol("a"), wrappedValue{bass.Int(1)}},
		{bass.Symbol("b"), bass.Bool(true)},
	}

	wrappedKA := bass.Assoc{
		{wrappedValue{bass.Symbol("a")}, bass.Int(1)},
		{bass.Symbol("b"), bass.Bool(true)},
	}

	wrappedB := bass.Assoc{
		{bass.Symbol("a"), bass.Int(1)},
		{bass.Symbol("b"), wrappedValue{bass.Bool(true)}},
	}

	differentA := bass.Assoc{
		{bass.Symbol("a"), bass.Int(2)},
		{bass.Symbol("b"), bass.Bool(true)},
	}

	differentB := bass.Assoc{
		{bass.Symbol("a"), bass.Int(1)},
		{bass.Symbol("b"), bass.Bool(false)},
	}

	missingA := bass.Assoc{
		{bass.Symbol("b"), bass.Bool(false)},
	}

	val := bass.NewEnv()
	require.True(t, obj.Equal(reverse))
	require.True(t, reverse.Equal(obj))
	require.True(t, obj.Equal(wrappedKA))
	require.True(t, obj.Equal(wrappedVA))
	require.True(t, obj.Equal(wrappedB))
	require.True(t, wrappedKA.Equal(obj))
	require.True(t, wrappedVA.Equal(obj))
	require.True(t, wrappedB.Equal(obj))
	require.False(t, obj.Equal(differentA))
	require.False(t, obj.Equal(differentA))
	require.False(t, differentA.Equal(obj))
	require.False(t, differentB.Equal(obj))
	require.False(t, missingA.Equal(obj))
	require.False(t, obj.Equal(missingA))
	require.False(t, val.Equal(bass.Null{}))
}

func TestAssocEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Assoc{
		{bass.Keyword("a"), bass.Int(1)},
		{bass.Symbol("key"), bass.Bool(true)},
		{bass.Keyword("c"), bass.Symbol("value")},
	}

	env.Set("key", bass.Keyword("b"))
	env.Set("value", bass.String("three"))

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, bass.Object{
		"a": bass.Int(1),
		"b": bass.Bool(true),
		"c": bass.String("three"),
	}, res)

	env.Set("key", bass.String("non-key"))

	res, err = val.Eval(env)
	require.ErrorIs(t, err, bass.BadKeyError{
		Value: bass.String("non-key"),
	})
}
