package bass_test

import (
	"testing"

	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/is"
)

func TestOperativeDecode(t *testing.T) {
	is := is.New(t)

	val := operative

	var c bass.Combiner
	err := val.Decode(&c)
	is.NoErr(err)
	is.Equal(c, val)

	var o *bass.Operative
	err = val.Decode(&o)
	is.NoErr(err)
	is.Equal(o, val)
}

func TestOperativeEqual(t *testing.T) {
	is := is.New(t)

	val := operative
	is.True(val.Equal(val))

	other := *operative
	is.True(!val.Equal(&other))
}

func TestOperativeCall(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	val := operative

	scope.Set("foo", bass.Int(42))

	res, err := Call(val, scope, bass.NewList(bass.Symbol("foo")))
	is.NoErr(err)
	is.Equal(

		res, bass.Pair{
			A: bass.Symbol("foo"),
			D: scope,
		})

}
