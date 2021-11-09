package bass_test

import (
	"testing"

	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/is"
)

func TestBoolDecode(t *testing.T) {
	is := is.New(t)

	var foo bool
	err := bass.Bool(true).Decode(&foo)
	is.NoErr(err)
	is.Equal(true, foo)

	err = bass.Bool(false).Decode(&foo)
	is.NoErr(err)
	is.Equal(false, foo)

	var b bass.Bool
	err = bass.Bool(true).Decode(&b)
	is.NoErr(err)
	is.Equal(b, bass.Bool(true))
}

func TestBoolEqual(t *testing.T) {
	is := is.New(t)

	Equal(t, bass.Bool(true), bass.Bool(true))
	Equal(t, bass.Bool(false), bass.Bool(false))
	is.True(!bass.Bool(true).Equal(bass.Bool(false)))
	Equal(t, bass.Bool(true), wrappedValue{bass.Bool(true)})
	is.True(!bass.Bool(true).Equal(wrappedValue{bass.Bool(false)}))
}
