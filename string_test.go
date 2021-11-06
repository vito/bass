package bass_test

import (
	"testing"

	"github.com/vito/bass"
	"github.com/vito/is"
)

func TestStringDecode(t *testing.T) {
	is := is.New(t)

	var foo string
	err := bass.String("foo").Decode(&foo)
	is.NoErr(err)
	is.Equal("foo", foo)

	err = bass.String("bar").Decode(&foo)
	is.NoErr(err)
	is.Equal("bar", foo)

	var str bass.String
	err = bass.String("bar").Decode(&str)
	is.NoErr(err)
	is.Equal(bass.String("bar"), str)
}

func TestStringEqual(t *testing.T) {
	is := is.New(t)

	is.True(bass.String("hello").Equal(bass.String("hello")))
	is.True(bass.String("").Equal(bass.String("")))
	is.True(!bass.String("hello").Equal(bass.String("")))
	is.True(bass.String("hello").Equal(wrappedValue{bass.String("hello")}))
	is.True(!bass.String("hello").Equal(wrappedValue{bass.String("")}))
}
