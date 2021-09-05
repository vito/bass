package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
)

func TestApplicativeDecode(t *testing.T) {
	val := bass.Wrapped{
		Underlying: recorderOp{},
	}

	var a bass.Wrapped
	err := val.Decode(&a)
	require.NoError(t, err)
	require.Equal(t, val, a)

	var c bass.Combiner
	err = val.Decode(&c)
	require.NoError(t, err)
	require.Equal(t, val, c)
}

func TestApplicativeEqual(t *testing.T) {
	val := bass.Wrapped{
		Underlying: recorderOp{},
	}

	require.True(t, val.Equal(val))
	require.False(t, val.Equal(noopFn))
}

func TestApplicativeCall(t *testing.T) {
	scope := bass.NewEmptyScope()
	val := bass.Wrapped{
		Underlying: recorderOp{},
	}

	scope.Def("foo", bass.Int(42))

	res, err := Call(val, scope, bass.NewList(bass.NewSymbol("foo")))
	require.NoError(t, err)
	require.Equal(t, recorderOp{
		Applied: bass.NewList(bass.Int(42)),
		Scope:   scope,
	}, res)

	res, err = Call(val, scope, bass.NewSymbol("foo"))
	require.NoError(t, err)
	require.Equal(t, recorderOp{
		Applied: bass.Int(42),
		Scope:   scope,
	}, res)
}
