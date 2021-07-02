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

	var b bass.Bool
	err = bass.Bool(true).Decode(&b)
	require.NoError(t, err)
	require.Equal(t, bass.Bool(true), b)
}

func TestBoolEqual(t *testing.T) {
	require.True(t, bass.Bool(true).Equal(bass.Bool(true)))
	require.True(t, bass.Bool(false).Equal(bass.Bool(false)))
	require.False(t, bass.Bool(true).Equal(bass.Bool(false)))
	require.True(t, bass.Bool(true).Equal(wrappedValue{bass.Bool(true)}))
	require.False(t, bass.Bool(true).Equal(wrappedValue{bass.Bool(false)}))
}
