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
