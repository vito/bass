package bass_test

import (
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
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

	is.True(bass.Int(42).Equal(bass.Int(42)))
	is.True(bass.Int(0).Equal(bass.Int(0)))
	is.True(!bass.Int(42).Equal(bass.Int(0)))
	is.True(bass.Int(42).Equal(wrappedValue{bass.Int(42)}))
	is.True(!bass.Int(42).Equal(wrappedValue{bass.Int(0)}))
}
