package bass_test

import (
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
)

func TestPairDecode(t *testing.T) {
	is := is.New(t)

	list := bass.NewList(
		bass.Int(1),
		bass.Bool(true),
		bass.String("three"),
	)

	var dest bass.List
	err := list.Decode(&dest)
	is.NoErr(err)
	is.Equal(dest, list)

	var pair bass.Pair
	err = list.Decode(&pair)
	is.NoErr(err)
	is.Equal(pair, list)

	var vals []bass.Value
	err = list.Decode(&vals)
	is.NoErr(err)
	is.Equal(

		vals, []bass.Value{
			bass.Int(1),
			bass.Bool(true),
			bass.String("three"),
		})

	intsList := bass.NewList(
		bass.Int(1),
		bass.Int(2),
		bass.Int(3),
	)

	var ints []int
	err = intsList.Decode(&ints)
	is.NoErr(err)
	is.Equal(

		ints, []int{
			1,
			2,
			3,
		})

}

func TestPairEqual(t *testing.T) {
	is := is.New(t)

	pair := bass.Pair{
		A: bass.Int(1),
		D: bass.Bool(true),
	}

	wrappedA := bass.Pair{
		A: wrappedValue{bass.Int(1)},
		D: bass.Bool(true),
	}

	wrappedD := bass.Pair{
		A: bass.Int(1),
		D: wrappedValue{bass.Bool(true)},
	}

	differentA := bass.Pair{
		A: bass.Int(2),
		D: bass.Bool(true),
	}

	differentD := bass.Pair{
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

	// not equal to Cons
	is.True(!pair.Equal(bass.Cons(pair)))
	is.True(!bass.Cons(pair).Equal(pair))
}

func TestPairListInterface(t *testing.T) {
	is := is.New(t)

	var list bass.List = bass.Pair{bass.Int(1), bass.Bool(true)}
	is.Equal(bass.Int(1), list.First())
	is.Equal(bass.Bool(true), list.Rest())
}
