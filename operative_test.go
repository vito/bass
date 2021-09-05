package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
)

func TestOperativeDecode(t *testing.T) {
	val := operative

	var c bass.Combiner
	err := val.Decode(&c)
	require.NoError(t, err)
	require.Equal(t, val, c)

	var o *bass.Operative
	err = val.Decode(&o)
	require.NoError(t, err)
	require.Equal(t, val, o)
}

func TestOperativeEqual(t *testing.T) {
	val := operative
	require.True(t, val.Equal(val))

	other := *operative
	require.False(t, val.Equal(&other))
}

func TestOperativeCall(t *testing.T) {
	scope := bass.NewEmptyScope()
	val := operative

	scope.Def("foo", bass.Int(42))

	res, err := Call(val, scope, bass.NewList(bass.NewSymbol("foo")))
	require.NoError(t, err)
	require.Equal(t, bass.Pair{
		A: bass.NewSymbol("foo"),
		D: scope,
	}, res)
}
