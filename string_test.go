package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestStringDecode(t *testing.T) {
	var foo string
	err := bass.String("foo").Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, "foo")

	err = bass.String("bar").Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, "bar")
}
