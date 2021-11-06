package bass_test

import (
	"testing"

	"github.com/vito/bass"
	"github.com/vito/is"
)

func TestBindDecode(t *testing.T) {
	is := is.New(t)

	list := bass.Bind{
		bass.Keyword("a"), bass.Int(1),
		bass.Keyword("b"), bass.Bool(true),
		bass.Keyword("c"), bass.String("three"),
	}

	var obj bass.Bind
	err := list.Decode(&obj)
	is.NoErr(err)
	is.Equal(obj, list)
}

func TestBindEqual(t *testing.T) {
	is := is.New(t)

	obj := bass.Bind{
		bass.Symbol("a"), bass.Int(1),
		bass.Symbol("b"), bass.Bool(true),
	}

	reverse := bass.Bind{
		bass.Symbol("a"), bass.Int(1),
		bass.Symbol("b"), bass.Bool(true),
	}

	wrappedVA := bass.Bind{
		bass.Symbol("a"), wrappedValue{bass.Int(1)},
		bass.Symbol("b"), bass.Bool(true),
	}

	wrappedKA := bass.Bind{
		wrappedValue{bass.Symbol("a")}, bass.Int(1),
		bass.Symbol("b"), bass.Bool(true),
	}

	wrappedB := bass.Bind{
		bass.Symbol("a"), bass.Int(1),
		bass.Symbol("b"), wrappedValue{bass.Bool(true)},
	}

	differentA := bass.Bind{
		bass.Symbol("a"), bass.Int(2),
		bass.Symbol("b"), bass.Bool(true),
	}

	differentB := bass.Bind{
		bass.Symbol("a"), bass.Int(1),
		bass.Symbol("b"), bass.Bool(false),
	}

	missingA := bass.Bind{
		bass.Symbol("b"), bass.Bool(false),
	}

	val := bass.NewEmptyScope()
	is.True(obj.Equal(reverse))
	is.True(reverse.Equal(obj))
	is.True(obj.Equal(wrappedKA))
	is.True(obj.Equal(wrappedVA))
	is.True(obj.Equal(wrappedB))
	is.True(wrappedKA.Equal(obj))
	is.True(wrappedVA.Equal(obj))
	is.True(wrappedB.Equal(obj))
	is.True(!obj.Equal(differentA))
	is.True(!obj.Equal(differentA))
	is.True(!differentA.Equal(obj))
	is.True(!differentB.Equal(obj))
	is.True(!missingA.Equal(obj))
	is.True(!obj.Equal(missingA))
	is.True(!val.Equal(bass.Null{}))
}
