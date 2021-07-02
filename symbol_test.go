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

	var sym bass.Symbol
	err = bass.Symbol("foo").Decode(&sym)
	require.NoError(t, err)
	require.Equal(t, sym, bass.Symbol("foo"))
}

func TestSymbolEqual(t *testing.T) {
	require.True(t, bass.Symbol("hello").Equal(bass.Symbol("hello")))
	require.False(t, bass.Symbol("hello").Equal(bass.String("hello")))
	require.True(t, bass.Symbol("hello").Equal(wrappedValue{bass.Symbol("hello")}))
	require.False(t, bass.Symbol("hello").Equal(wrappedValue{bass.String("hello")}))
}
