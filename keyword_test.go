package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestKeywordDecode(t *testing.T) {
	var sym bass.Keyword
	err := bass.Keyword("foo").Decode(&sym)
	require.NoError(t, err)
	require.Equal(t, bass.Keyword("foo"), sym)

	var comb bass.Combiner
	err = bass.Keyword("foo").Decode(&comb)
	require.NoError(t, err)
	require.Equal(t, bass.Keyword("foo"), comb)
}

func TestKeywordEqual(t *testing.T) {
	require.True(t, bass.Keyword("hello").Equal(bass.Keyword("hello")))
	require.False(t, bass.Keyword("hello").Equal(bass.String("hello")))
	require.True(t, bass.Keyword("hello").Equal(wrappedValue{bass.Keyword("hello")}))
	require.False(t, bass.Keyword("hello").Equal(wrappedValue{bass.String("hello")}))
}

func TestKeywordCall(t *testing.T) {
	env := bass.NewEnv()
	env.Set("obj", bass.Object{"foo": bass.Int(42)})
	env.Set("def", bass.String("default"))

	res, err := Call(bass.Keyword("foo"), env, bass.NewList(bass.Symbol("obj")))
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)

	res, err = Call(bass.Keyword("bar"), env, bass.NewList(bass.Symbol("obj")))
	require.NoError(t, err)
	require.Equal(t, bass.Null{}, res)

	res, err = Call(
		bass.Keyword("bar"),
		env,
		bass.NewList(
			bass.Symbol("obj"),
			bass.Symbol("def"),
		),
	)
	require.NoError(t, err)
	require.Equal(t, bass.String("default"), res)
}
