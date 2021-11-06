package bass_test

import (
	"testing"

	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/is"
)

func TestApplicativeDecode(t *testing.T) {
	is := is.New(t)

	val := bass.Wrapped{
		Underlying: recorderOp{},
	}

	var a bass.Wrapped
	err := val.Decode(&a)
	is.NoErr(err)
	is.Equal(a, val)

	var c bass.Combiner
	err = val.Decode(&c)
	is.NoErr(err)
	is.Equal(c, val)
}

func TestApplicativeEqual(t *testing.T) {
	is := is.New(t)

	val := bass.Wrapped{
		Underlying: recorderOp{},
	}

	is.True(val.Equal(val))
	is.True(!val.Equal(noopFn))
}

func TestApplicativeCall(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	val := bass.Wrapped{
		Underlying: recorderOp{},
	}

	scope.Set("foo", bass.Int(42))

	res, err := Call(val, scope, bass.NewList(bass.Symbol("foo")))
	is.NoErr(err)
	is.Equal(

		res, recorderOp{
			Applied: bass.NewList(bass.Int(42)),
			Scope:   scope,
		})

	res, err = Call(val, scope, bass.Symbol("foo"))
	is.NoErr(err)
	is.Equal(

		res, recorderOp{
			Applied: bass.Int(42),
			Scope:   scope,
		})

}
