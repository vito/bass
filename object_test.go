package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestObjectDecode(t *testing.T) {
	list := bass.Object{
		"a": bass.Int(1),
		"b": bass.Bool(true),
		"c": bass.String("three"),
	}

	var obj bass.Object
	err := list.Decode(&obj)
	require.NoError(t, err)
	require.Equal(t, list, obj)
}

func TestObjectEqual(t *testing.T) {
	obj := bass.Object{
		"a": bass.Int(1),
		"b": bass.Bool(true),
	}

	wrappedA := bass.Object{
		"a": wrappedValue{bass.Int(1)},
		"b": bass.Bool(true),
	}

	wrappedB := bass.Object{
		"a": bass.Int(1),
		"b": wrappedValue{bass.Bool(true)},
	}

	differentA := bass.Object{
		"a": bass.Int(2),
		"b": bass.Bool(true),
	}

	differentB := bass.Object{
		"a": bass.Int(1),
		"b": bass.Bool(false),
	}

	missingA := bass.Object{
		"b": bass.Bool(true),
	}

	val := bass.NewEnv()
	require.True(t, obj.Equal(wrappedA))
	require.True(t, obj.Equal(wrappedB))
	require.True(t, wrappedA.Equal(obj))
	require.True(t, wrappedB.Equal(obj))
	require.False(t, obj.Equal(differentA))
	require.False(t, obj.Equal(differentA))
	require.False(t, differentA.Equal(obj))
	require.False(t, differentB.Equal(obj))
	require.False(t, missingA.Equal(obj))
	require.False(t, obj.Equal(missingA))
	require.False(t, val.Equal(bass.Null{}))
}
