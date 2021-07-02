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

	var str bass.String
	err = bass.String("bar").Decode(&str)
	require.NoError(t, err)
	require.Equal(t, str, bass.String("bar"))
}

func TestStringEqual(t *testing.T) {
	require.True(t, bass.String("hello").Equal(bass.String("hello")))
	require.True(t, bass.String("").Equal(bass.String("")))
	require.False(t, bass.String("hello").Equal(bass.String("")))
	require.True(t, bass.String("hello").Equal(wrappedValue{bass.String("hello")}))
	require.False(t, bass.String("hello").Equal(wrappedValue{bass.String("")}))
}
