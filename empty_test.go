package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestEmptyDecode(t *testing.T) {
	var dest bass.List
	err := bass.Empty{}.Decode(&dest)
	require.NoError(t, err)
	require.Equal(t, bass.Empty{}, dest)

	var empty bass.Empty
	err = bass.Empty{}.Decode(&dest)
	require.NoError(t, err)
	require.Equal(t, bass.Empty{}, empty)

	var vals []bass.Value
	err = bass.Empty{}.Decode(&vals)
	require.NoError(t, err)
	require.Equal(t, []bass.Value{}, vals)

	vals = []bass.Value{bass.Int(1), bass.Int(2), bass.Int(3)}
	err = bass.Empty{}.Decode(&vals)
	require.NoError(t, err)
	require.Equal(t, []bass.Value{}, vals)
}

func TestEmptyEqual(t *testing.T) {
	require.True(t, bass.Empty{}.Equal(bass.Empty{}))
	require.True(t, bass.Empty{}.Equal(wrappedValue{bass.Empty{}}))
	require.False(t, bass.Empty{}.Equal(bass.Null{}))
}
