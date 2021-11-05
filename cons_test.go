package bass_test

import (
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
)

func TestConsDecode(t *testing.T) {
	is := is.New(t)

	pair := bass.Cons{
		A: bass.Int(1),
		D: bass.Pair{
			A: bass.Bool(true),
			D: bass.String("three"),
		},
	}

	var dest bass.List
	err := pair.Decode(&dest)
	is.NoErr(err)
	is.Equal(dest, pair)
}

func TestConsEqual(t *testing.T) {
	is := is.New(t)

	pair := bass.Cons{
		A: bass.Int(1),
		D: bass.Bool(true),
	}

	wrappedA := bass.Cons{
		A: wrappedValue{bass.Int(1)},
		D: bass.Bool(true),
	}

	wrappedD := bass.Cons{
		A: bass.Int(1),
		D: wrappedValue{bass.Bool(true)},
	}

	differentA := bass.Cons{
		A: bass.Int(2),
		D: bass.Bool(true),
	}

	differentD := bass.Cons{
		A: bass.Int(1),
		D: bass.Bool(false),
	}

	val := bass.NewEmptyScope()
	is.True(pair.Equal(wrappedA))
	is.True(pair.Equal(wrappedD))
	is.True(wrappedA.Equal(pair))
	is.True(wrappedD.Equal(pair))
	is.True(!pair.Equal(differentA))
	is.True(!pair.Equal(differentA))
	is.True(!differentA.Equal(pair))
	is.True(!differentD.Equal(pair))
	is.True(!val.Equal(bass.Null{}))

	// not equal to Pair
	is.True(!pair.Equal(bass.Pair(pair)))
	is.True(!bass.Pair(pair).Equal(pair))
}

func TestConsListInterface(t *testing.T) {
	is := is.New(t)

	var list bass.List = bass.Cons{bass.Int(1), bass.Bool(true)}
	is.Equal(bass.Int(1), list.First())
	is.Equal(bass.Bool(true), list.Rest())
}
