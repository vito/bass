package bass_test

import (
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
)

func TestAnnotatedDecode(t *testing.T) {
	is := is.New(t)

	val := bass.Annotated{
		Comment: "hello",
		Value: dummyValue{
			sentinel: 42,
		},
	}

	var dest dummyValue
	err := val.Decode(&dest)
	is.NoErr(err)
	is.Equal(dest, val.Value)
}

func TestAnnotatedEqual(t *testing.T) {
	is := is.New(t)

	val := bass.Annotated{
		Comment: "hello",
		Value: dummyValue{
			sentinel: 42,
		},
	}

	is.True(val.Equal(val))
	is.True(!val.Equal(bass.Annotated{
		Comment: "hello",
		Value: dummyValue{
			sentinel: 43,
		},
	}))

	// compare inner value only
	is.True(val.Equal(bass.Annotated{
		Comment: "different",
		Value: dummyValue{
			sentinel: 42,
		},
	}))

}
