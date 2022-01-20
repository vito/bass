package bass_test

import (
	"testing"

	"github.com/vito/bass/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/is"
)

func TestNullDecode(t *testing.T) {
	is := is.New(t)

	var n bass.Null
	err := bass.Null{}.Decode(&n)
	is.NoErr(err)
	is.Equal(bass.Null{}, n)

	var foo string
	err = bass.Null{}.Decode(&foo)
	is.True(err != nil)

	var b bool = true
	err = bass.Null{}.Decode(&b)
	is.NoErr(err)
	is.True(!b)
}

func TestNullEqual(t *testing.T) {
	is := is.New(t)

	Equal(t, bass.Null{}, bass.Null{})
	Equal(t, bass.Null{}, wrappedValue{bass.Null{}})
	is.True(!bass.Null{}.Equal(bass.Bool(false)))
}
