package bass_test

import (
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
)

func TestSymbolDecode(t *testing.T) {
	is := is.New(t)

	var sym bass.Symbol
	err := bass.Symbol("foo").Decode(&sym)
	is.NoErr(err)
	is.Equal(bass.Symbol("foo"), sym)

	var foo string
	err = bass.Symbol("bar").Decode(&foo)
	is.True(err != nil)

	var bar bass.String
	err = bass.Symbol("bar").Decode(&bar)
	is.True(err != nil)

	var comb bass.Combiner
	err = bass.Symbol("foo").Decode(&comb)
	is.NoErr(err)
	is.Equal(comb, bass.Symbol("foo"))

	var app bass.Applicative
	err = bass.Symbol("foo").Decode(&app)
	is.NoErr(err)
	is.Equal(app, bass.Symbol("foo"))
}

func TestSymbolEqual(t *testing.T) {
	is := is.New(t)

	is.True(bass.Symbol("hello").Equal(bass.Symbol("hello")))
	is.True(!bass.Symbol("hello").Equal(bass.String("hello")))
	is.True(bass.Symbol("hello").Equal(wrappedValue{bass.Symbol("hello")}))
	is.True(!bass.Symbol("hello").Equal(wrappedValue{bass.String("hello")}))
}

func TestSymbolOperativeEqual(t *testing.T) {
	is := is.New(t)

	op := bass.Symbol("hello").Unwrap()
	is.True(op.Equal(bass.Symbol("hello").Unwrap()))
	is.True(!op.Equal(bass.Symbol("goodbye").Unwrap()))
	is.True(op.Equal(wrappedValue{bass.Symbol("hello").Unwrap()}))
	is.True(!op.Equal(wrappedValue{bass.Symbol("goodbye").Unwrap()}))
}

func TestSymbolCallScope(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	scope.Set("foo", bass.Int(42))
	scope.Set("def", bass.String("default"))
	scope.Set("self", scope)

	res, err := Call(bass.Symbol("foo"), scope, bass.NewList(bass.Symbol("self")))
	is.NoErr(err)
	is.Equal(res, bass.Int(42))

	res, err = Call(bass.Symbol("bar"), scope, bass.NewList(bass.Symbol("self")))
	is.NoErr(err)
	is.Equal(res, bass.Null{})

	res, err = Call(
		bass.Symbol("bar"),
		scope,
		bass.NewList(
			bass.Symbol("self"),
			bass.Symbol("def"),
		),
	)
	is.NoErr(err)
	is.Equal(res, bass.String("default"))
}

func TestSymbolUnwrap(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	target := bass.Bindings{"foo": bass.Int(42)}.Scope()
	def := bass.String("default")

	res, err := Call(bass.Symbol("foo").Unwrap(), scope, bass.NewList(target))
	is.NoErr(err)
	is.Equal(res, bass.Int(42))

	res, err = Call(bass.Symbol("bar").Unwrap(), scope, bass.NewList(target))
	is.NoErr(err)
	is.Equal(res, bass.Null{})

	res, err = Call(
		bass.Symbol("bar"),
		scope,
		bass.NewList(
			target,
			def,
		),
	)
	is.NoErr(err)
	is.Equal(res, bass.String("default"))
}
