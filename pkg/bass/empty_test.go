package bass_test

import (
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
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

	Equal(t, bass.Empty{}, bass.Empty{})
	Equal(t, bass.Empty{}, wrappedValue{bass.Empty{}})
	is.True(!bass.Empty{}.Equal(bass.Null{}))
}
