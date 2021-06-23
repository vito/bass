package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

// func TestListDecode(t *testing.T) {
// 	values := []bass.Value{
// 		bass.Int(1),
// 		bass.Bool(true),
// 		bass.String("three"),
// 	}

// 	var dest []bass.Value
// 	err := bass.NewList(values...).Decode(&dest)
// 	require.NoError(t, err)
// 	require.Equal(t, dest, values)

// 	err = bass.NewList().Decode(&dest)
// 	require.NoError(t, err)
// 	require.Empty(t, dest)
// }

func TestListInterface(t *testing.T) {
	var list bass.List = bass.Empty{}
	require.Equal(t, list.First(), bass.Empty{})
	require.Equal(t, list.Rest(), bass.Empty{})

	list = bass.Pair{bass.Int(1), bass.Bool(true)}
	require.Equal(t, list.First(), bass.Int(1))
	require.Equal(t, list.Rest(), bass.Bool(true))
}
