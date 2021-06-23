package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestSymbolDecode(t *testing.T) {
	var foo string
	err := bass.Symbol("foo").Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, "foo")

	err = bass.Symbol("bar").Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, foo, "bar")
}
