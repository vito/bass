package bass_test

import (
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
)

func TestEmptyDecode(t *testing.T) {
	is := is.New(t)

	var dest bass.List
	err := bass.Empty{}.Decode(&dest)
	is.NoErr(err)
	is.Equal(dest, bass.Empty{})

	var empty bass.Empty
	err = bass.Empty{}.Decode(&dest)
	is.NoErr(err)
	is.Equal(empty, bass.Empty{})

	var vals []bass.Value
	err = bass.Empty{}.Decode(&vals)
	is.NoErr(err)
	is.Equal(vals, []bass.Value{})

	vals = []bass.Value{bass.Int(1), bass.Int(2), bass.Int(3)}
	err = bass.Empty{}.Decode(&vals)
	is.NoErr(err)
	is.Equal(vals, []bass.Value{})
}

func TestEmptyEqual(t *testing.T) {
	is := is.New(t)

	is.True(bass.Empty{}.Equal(bass.Empty{}))
	is.True(bass.Empty{}.Equal(wrappedValue{bass.Empty{}}))
	is.True(!bass.Empty{}.Equal(bass.Null{}))
}
