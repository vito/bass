package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
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

	var app bass.Applicative
	err = bass.Keyword("foo").Decode(&app)
	require.NoError(t, err)
	require.Equal(t, bass.Keyword("foo"), app)
}

func TestKeywordEqual(t *testing.T) {
	require.True(t, bass.Keyword("hello").Equal(bass.Keyword("hello")))
	require.False(t, bass.Keyword("hello").Equal(bass.String("hello")))
	require.True(t, bass.Keyword("hello").Equal(wrappedValue{bass.Keyword("hello")}))
	require.False(t, bass.Keyword("hello").Equal(wrappedValue{bass.String("hello")}))
}

func TestKeywordOperativeEqual(t *testing.T) {
	op := bass.Keyword("hello").Unwrap()
	require.True(t, op.Equal(bass.Keyword("hello").Unwrap()))
	require.False(t, op.Equal(bass.Keyword("goodbye").Unwrap()))
	require.True(t, op.Equal(wrappedValue{bass.Keyword("hello").Unwrap()}))
	require.False(t, op.Equal(wrappedValue{bass.Keyword("goodbye").Unwrap()}))
}

func TestKeywordCallObject(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Set("obj", bass.Object{"foo": bass.Int(42)})
	scope.Set("def", bass.String("default"))

	res, err := Call(bass.Keyword("foo"), scope, bass.NewList(bass.Symbol("obj")))
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)

	res, err = Call(bass.Keyword("bar"), scope, bass.NewList(bass.Symbol("obj")))
	require.NoError(t, err)
	require.Equal(t, bass.Null{}, res)

	res, err = Call(
		bass.Keyword("bar"),
		scope,
		bass.NewList(
			bass.Symbol("obj"),
			bass.Symbol("def"),
		),
	)
	require.NoError(t, err)
	require.Equal(t, bass.String("default"), res)
}

func TestKeywordCallScope(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Set("foo", bass.Int(42))
	scope.Set("def", bass.String("default"))
	scope.Set("self", scope)

	res, err := Call(bass.Keyword("foo"), scope, bass.NewList(bass.Symbol("self")))
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)

	res, err = Call(bass.Keyword("bar"), scope, bass.NewList(bass.Symbol("self")))
	require.NoError(t, err)
	require.Equal(t, bass.Null{}, res)

	res, err = Call(
		bass.Keyword("bar"),
		scope,
		bass.NewList(
			bass.Symbol("self"),
			bass.Symbol("def"),
		),
	)
	require.NoError(t, err)
	require.Equal(t, bass.String("default"), res)
}

func TestKeywordUnwrap(t *testing.T) {
	scope := bass.NewEmptyScope()
	obj := bass.Object{"foo": bass.Int(42)}
	def := bass.String("default")

	res, err := Call(bass.Keyword("foo").Unwrap(), scope, bass.NewList(obj))
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)

	res, err = Call(bass.Keyword("bar").Unwrap(), scope, bass.NewList(obj))
	require.NoError(t, err)
	require.Equal(t, bass.Null{}, res)

	res, err = Call(
		bass.Keyword("bar"),
		scope,
		bass.NewList(
			obj,
			def,
		),
	)
	require.NoError(t, err)
	require.Equal(t, bass.String("default"), res)
}
