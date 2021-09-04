package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
)

func TestSymbolDecode(t *testing.T) {
	var sym bass.Symbol
	err := bass.Symbol("foo").Decode(&sym)
	require.NoError(t, err)
	require.Equal(t, sym, bass.Symbol("foo"))

	var foo string
	err = bass.Symbol("bar").Decode(&foo)
	require.Error(t, err)

	var bar bass.String
	err = bass.Symbol("bar").Decode(&bar)
	require.Error(t, err)

	var comb bass.Combiner
	err = bass.Symbol("foo").Decode(&comb)
	require.NoError(t, err)
	require.Equal(t, bass.Symbol("foo"), comb)

	var app bass.Applicative
	err = bass.Symbol("foo").Decode(&app)
	require.NoError(t, err)
	require.Equal(t, bass.Symbol("foo"), app)
}

func TestSymbolEqual(t *testing.T) {
	require.True(t, bass.Symbol("hello").Equal(bass.Symbol("hello")))
	require.False(t, bass.Symbol("hello").Equal(bass.String("hello")))
	require.True(t, bass.Symbol("hello").Equal(wrappedValue{bass.Symbol("hello")}))
	require.False(t, bass.Symbol("hello").Equal(wrappedValue{bass.String("hello")}))
}

func TestSymbolOperativeEqual(t *testing.T) {
	op := bass.Symbol("hello").Unwrap()
	require.True(t, op.Equal(bass.Symbol("hello").Unwrap()))
	require.False(t, op.Equal(bass.Symbol("goodbye").Unwrap()))
	require.True(t, op.Equal(wrappedValue{bass.Symbol("hello").Unwrap()}))
	require.False(t, op.Equal(wrappedValue{bass.Symbol("goodbye").Unwrap()}))
}

func TestSymbolCallScope(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Set("foo", bass.Int(42))
	scope.Set("def", bass.String("default"))
	scope.Set("self", scope)

	res, err := Call(bass.Symbol("foo"), scope, bass.NewList(bass.Symbol("self")))
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)

	res, err = Call(bass.Symbol("bar"), scope, bass.NewList(bass.Symbol("self")))
	require.NoError(t, err)
	require.Equal(t, bass.Null{}, res)

	res, err = Call(
		bass.Symbol("bar"),
		scope,
		bass.NewList(
			bass.Symbol("self"),
			bass.Symbol("def"),
		),
	)
	require.NoError(t, err)
	require.Equal(t, bass.String("default"), res)
}

func TestSymbolUnwrap(t *testing.T) {
	scope := bass.NewEmptyScope()
	target := bass.Bindings{"foo": bass.Int(42)}.Scope()
	def := bass.String("default")

	res, err := Call(bass.Symbol("foo").Unwrap(), scope, bass.NewList(target))
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)

	res, err = Call(bass.Symbol("bar").Unwrap(), scope, bass.NewList(target))
	require.NoError(t, err)
	require.Equal(t, bass.Null{}, res)

	res, err = Call(
		bass.Symbol("bar"),
		scope,
		bass.NewList(
			target,
			def,
		),
	)
	require.NoError(t, err)
	require.Equal(t, bass.String("default"), res)
}
