package bass_test

import (
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
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

	Equal(t, bass.String("hello"), bass.String("hello"))
	Equal(t, bass.String(""), bass.String(""))
	is.True(!bass.String("hello").Equal(bass.String("")))
	Equal(t, bass.String("hello"), wrappedValue{bass.String("hello")})
	is.True(!bass.String("hello").Equal(wrappedValue{bass.String("")}))
}
