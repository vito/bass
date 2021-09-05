package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestBindDecode(t *testing.T) {
	list := bass.Bind{
		bass.NewKeyword("a"), bass.Int(1),
		bass.NewKeyword("b"), bass.Bool(true),
		bass.NewKeyword("c"), bass.String("three"),
	}

	var obj bass.Bind
	err := list.Decode(&obj)
	require.NoError(t, err)
	require.Equal(t, list, obj)
}

func TestBindEqual(t *testing.T) {
	obj := bass.Bind{
		bass.NewSymbol("a"), bass.Int(1),
		bass.NewSymbol("b"), bass.Bool(true),
	}

	reverse := bass.Bind{
		bass.NewSymbol("a"), bass.Int(1),
		bass.NewSymbol("b"), bass.Bool(true),
	}

	wrappedVA := bass.Bind{
		bass.NewSymbol("a"), wrappedValue{bass.Int(1)},
		bass.NewSymbol("b"), bass.Bool(true),
	}

	wrappedKA := bass.Bind{
		wrappedValue{bass.NewSymbol("a")}, bass.Int(1),
		bass.NewSymbol("b"), bass.Bool(true),
	}

	wrappedB := bass.Bind{
		bass.NewSymbol("a"), bass.Int(1),
		bass.NewSymbol("b"), wrappedValue{bass.Bool(true)},
	}

	differentA := bass.Bind{
		bass.NewSymbol("a"), bass.Int(2),
		bass.NewSymbol("b"), bass.Bool(true),
	}

	differentB := bass.Bind{
		bass.NewSymbol("a"), bass.Int(1),
		bass.NewSymbol("b"), bass.Bool(false),
	}

	missingA := bass.Bind{
		bass.NewSymbol("b"), bass.Bool(false),
	}

	val := bass.NewEmptyScope()
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
