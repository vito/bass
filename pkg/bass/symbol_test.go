package bass_test

import (
	"errors"
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
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

	Equal(t, bass.Symbol("hello"), bass.Symbol("hello"))
	is.True(!bass.Symbol("hello").Equal(bass.String("hello")))
	Equal(t, bass.Symbol("hello"), wrappedValue{bass.Symbol("hello")})
	is.True(!bass.Symbol("hello").Equal(wrappedValue{bass.String("hello")}))
}

func TestSymbolOperativeEqual(t *testing.T) {
	is := is.New(t)

	op := bass.Symbol("hello").Unwrap()
	Equal(t, op, bass.Symbol("hello").Unwrap())
	is.True(!op.Equal(bass.Symbol("goodbye").Unwrap()))
	Equal(t, op, wrappedValue{bass.Symbol("hello").Unwrap()})
	is.True(!op.Equal(wrappedValue{bass.Symbol("goodbye").Unwrap()}))
}

func TestSymbolCallScope(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	scope.Set("bound", bass.Int(42))
	scope.Set("bar", bass.String("default"))
	scope.Set("self", scope)

	_, err := Call(bass.Symbol("bound"), scope, bass.Empty{})
	is.True(errors.Is(err, bass.ArityError{
		Name:     bass.Keyword("bound").Repr(),
		Need:     1,
		Have:     0,
		Variadic: true,
	}))

	res, err := Call(bass.Symbol("bound"), scope, bass.NewList(bass.Symbol("self")))
	is.NoErr(err)
	is.Equal(res, bass.Int(42))

	_, err = Call(bass.Symbol("unbound"), scope, bass.NewList(bass.Symbol("self")))
	is.Equal(err, bass.UnboundError{"unbound", scope})

	res, err = Call(
		bass.Symbol("unbound"),
		scope,
		bass.NewList(
			bass.Symbol("self"),
			bass.Symbol("bar"),
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

	_, err = Call(bass.Symbol("bar").Unwrap(), scope, bass.NewList(target))
	is.Equal(err, bass.UnboundError{"bar", target})

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
