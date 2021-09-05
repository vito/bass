package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
)

func TestSymbolDecode(t *testing.T) {
	var sym bass.Symbol
	err := bass.NewSymbol("foo").Decode(&sym)
	require.NoError(t, err)
	require.Equal(t, sym, bass.NewSymbol("foo"))

	var foo string
	err = bass.NewSymbol("bar").Decode(&foo)
	require.Error(t, err)

	var bar bass.String
	err = bass.NewSymbol("bar").Decode(&bar)
	require.Error(t, err)

	var comb bass.Combiner
	err = bass.NewSymbol("foo").Decode(&comb)
	require.NoError(t, err)
	require.Equal(t, bass.NewSymbol("foo"), comb)

	var app bass.Applicative
	err = bass.NewSymbol("foo").Decode(&app)
	require.NoError(t, err)
	require.Equal(t, bass.NewSymbol("foo"), app)
}

func TestSymbolEqual(t *testing.T) {
	require.True(t, bass.NewSymbol("hello").Equal(bass.NewSymbol("hello")))
	require.False(t, bass.NewSymbol("hello").Equal(bass.String("hello")))
	require.True(t, bass.NewSymbol("hello").Equal(wrappedValue{bass.NewSymbol("hello")}))
	require.False(t, bass.NewSymbol("hello").Equal(wrappedValue{bass.String("hello")}))
}

func TestSymbolOperativeEqual(t *testing.T) {
	op := bass.NewSymbol("hello").Unwrap()
	require.True(t, op.Equal(bass.NewSymbol("hello").Unwrap()))
	require.False(t, op.Equal(bass.NewSymbol("goodbye").Unwrap()))
	require.True(t, op.Equal(wrappedValue{bass.NewSymbol("hello").Unwrap()}))
	require.False(t, op.Equal(wrappedValue{bass.NewSymbol("goodbye").Unwrap()}))
}

func TestSymbolCallScope(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Def("foo", bass.Int(42))
	scope.Def("def", bass.String("default"))
	scope.Def("self", scope)

	res, err := Call(bass.NewSymbol("foo"), scope, bass.NewList(bass.NewSymbol("self")))
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)

	res, err = Call(bass.NewSymbol("bar"), scope, bass.NewList(bass.NewSymbol("self")))
	require.NoError(t, err)
	require.Equal(t, bass.Null{}, res)

	res, err = Call(
		bass.NewSymbol("bar"),
		scope,
		bass.NewList(
			bass.NewSymbol("self"),
			bass.NewSymbol("def"),
		),
	)
	require.NoError(t, err)
	require.Equal(t, bass.String("default"), res)
}

func TestSymbolUnwrap(t *testing.T) {
	scope := bass.NewEmptyScope()
	target := bass.Bindings{"foo": bass.Int(42)}.Scope()
	def := bass.String("default")

	res, err := Call(bass.NewSymbol("foo").Unwrap(), scope, bass.NewList(target))
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)

	res, err = Call(bass.NewSymbol("bar").Unwrap(), scope, bass.NewList(target))
	require.NoError(t, err)
	require.Equal(t, bass.Null{}, res)

	res, err = Call(
		bass.NewSymbol("bar"),
		scope,
		bass.NewList(
			target,
			def,
		),
	)
	require.NoError(t, err)
	require.Equal(t, bass.String("default"), res)
}
