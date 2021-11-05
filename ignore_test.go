package bass_test

import (
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
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

	is.True(bass.Ignore{}.Equal(bass.Ignore{}))
	is.True(bass.Ignore{}.Equal(wrappedValue{bass.Ignore{}}))
	is.True(!bass.Ignore{}.Equal(bass.Null{}))
}
