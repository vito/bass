package bass_test

import (
	"testing"

	"github.com/vito/bass/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/is"
)

func TestIgnoreDecode(t *testing.T) {
	is := is.New(t)

	ign := bass.Ignore{}
	err := bass.Ignore{}.Decode(&ign)
	is.NoErr(err)
	is.Equal(bass.Ignore{}, ign)

	str := "some untouched value"
	err = bass.Ignore{}.Decode(&str)
	is.Equal(

		err, bass.DecodeError{
			Source:      bass.Ignore{},
			Destination: &str,
		})

}

func TestIgnoreEqual(t *testing.T) {
	is := is.New(t)

	Equal(t, bass.Ignore{}, bass.Ignore{})
	Equal(t, bass.Ignore{}, wrappedValue{bass.Ignore{}})
	is.True(!bass.Ignore{}.Equal(bass.Null{}))
}
