package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestNullDecode(t *testing.T) {
	var foo string
	err := bass.Null{}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, "")

	foo = "some stale value"
	err = bass.Null{}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, "")
}
