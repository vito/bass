package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestIntDecode(t *testing.T) {
	var foo int
	err := bass.Int(42).Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, 42)

	err = bass.Int(0).Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, 0)
}
