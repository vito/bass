package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestPairDecode(t *testing.T) {
	list := bass.NewList(
		bass.Int(1),
		bass.Bool(true),
		bass.String("three"),
	)

	var dest bass.List
	err := list.Decode(&dest)
	require.NoError(t, err)
	require.Equal(t, list, dest)

	var pair bass.Pair
	err = list.Decode(&pair)
	require.NoError(t, err)
	require.Equal(t, list, pair)

	var vals []bass.Value
	err = list.Decode(&vals)
	require.NoError(t, err)
	require.Equal(t, []bass.Value{
		bass.Int(1),
		bass.Bool(true),
		bass.String("three"),
	}, vals)

	intsList := bass.NewList(
		bass.Int(1),
		bass.Int(2),
		bass.Int(3),
	)

	var ints []int
	err = intsList.Decode(&ints)
	require.NoError(t, err)
	require.Equal(t, []int{
		1,
		2,
		3,
	}, ints)
}

func TestPairEqual(t *testing.T) {
	pair := bass.Pair{
		A: bass.Int(1),
		D: bass.Bool(true),
	}

	wrappedA := bass.Pair{
		A: wrappedValue{bass.Int(1)},
		D: bass.Bool(true),
	}

	wrappedD := bass.Pair{
		A: bass.Int(1),
		D: wrappedValue{bass.Bool(true)},
	}

	differentA := bass.Pair{
		A: bass.Int(2),
		D: bass.Bool(true),
	}

	differentD := bass.Pair{
		A: bass.Int(1),
		D: bass.Bool(false),
	}

	val := bass.NewScope()
	require.True(t, pair.Equal(wrappedA))
	require.True(t, pair.Equal(wrappedD))
	require.True(t, wrappedA.Equal(pair))
	require.True(t, wrappedD.Equal(pair))
	require.False(t, pair.Equal(differentA))
	require.False(t, pair.Equal(differentA))
	require.False(t, differentA.Equal(pair))
	require.False(t, differentD.Equal(pair))
	require.False(t, val.Equal(bass.Null{}))

	// not equal to Cons
	require.False(t, pair.Equal(bass.Cons(pair)))
	require.False(t, bass.Cons(pair).Equal(pair))
}

func TestPairListInterface(t *testing.T) {
	var list bass.List = bass.Pair{bass.Int(1), bass.Bool(true)}
	require.Equal(t, list.First(), bass.Int(1))
	require.Equal(t, list.Rest(), bass.Bool(true))
}
