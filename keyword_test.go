package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestKeywordDecode(t *testing.T) {
	var sym bass.Keyword
	err := bass.NewKeyword("foo").Decode(&sym)
	require.NoError(t, err)
	require.Equal(t, bass.NewKeyword("foo"), sym)
}

func TestKeywordEqual(t *testing.T) {
	require.True(t, bass.NewKeyword("hello").Equal(bass.NewKeyword("hello")))
	require.False(t, bass.NewKeyword("hello").Equal(bass.String("hello")))
	require.True(t, bass.NewKeyword("hello").Equal(wrappedValue{bass.NewKeyword("hello")}))
	require.False(t, bass.NewKeyword("hello").Equal(wrappedValue{bass.String("hello")}))
}
