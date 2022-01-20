package bass_test

import (
	"testing"

	"github.com/vito/bass/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/is"
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
	Equal(t, pair, wrappedA)
	Equal(t, pair, wrappedD)
	Equal(t, wrappedA, pair)
	Equal(t, wrappedD, pair)
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
