package bass_test

import (
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
)

func TestIntDecode(t *testing.T) {
	is := is.New(t)

	var foo int
	err := bass.Int(42).Decode(&foo)
	is.NoErr(err)
	is.Equal(42, foo)

	err = bass.Int(0).Decode(&foo)
	is.NoErr(err)
	is.Equal(0, foo)

	var i bass.Int
	err = bass.Int(42).Decode(&i)
	is.NoErr(err)
	is.Equal(i, bass.Int(42))
}

func TestIntEqual(t *testing.T) {
	is := is.New(t)

	Equal(t, bass.Int(42), bass.Int(42))
	Equal(t, bass.Int(0), bass.Int(0))
	is.True(!bass.Int(42).Equal(bass.Int(0)))
	Equal(t, bass.Int(42), wrappedValue{bass.Int(42)})
	is.True(!bass.Int(42).Equal(wrappedValue{bass.Int(0)}))
}
