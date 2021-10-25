package bass_test

import (
	"testing"

	"github.com/vito/bass"
	"github.com/vito/is"
)

func TestAnnotateDecode(t *testing.T) {
	is := is.New(t)

	val := bass.Annotate{
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

func TestAnnotatedJSON(t *testing.T) {
	is := is.New(t)

	val := bass.Annotated{
		Value: bass.Bindings{
			"a": bass.Bool(true),
			"b": bass.Int(1),
			"c": bass.String("hello"),
		}.Scope(),
		Meta: bass.Bindings{
			"bar": bass.Int(42),
		}.Scope(),
	}

	vp, err := bass.MarshalJSON(val.Value)
	is.NoErr(err)

	p, err := bass.MarshalJSON(val)
	is.NoErr(err)

	is.Equal(p, vp)
}

func TestAnnotateEqual(t *testing.T) {
	is := is.New(t)

	val := bass.Annotate{
		Comment: "hello",
		Value: dummyValue{
			sentinel: 42,
		},
	}

	is.True(val.Equal(val))
	is.True(!val.Equal(bass.Annotate{
		Comment: "hello",
		Value: dummyValue{
			sentinel: 43,
		},
	}))

	// compare inner value only
	is.True(val.Equal(bass.Annotate{
		Comment: "different",
		Value: dummyValue{
			sentinel: 42,
		},
	}))

}

func TestAnnotatedDecode(t *testing.T) {
	is := is.New(t)

	val := bass.Annotated{
		Value: dummyValue{
			sentinel: 42,
		},
		Meta: bass.Bindings{
			"doc": bass.String("hello"),
		}.Scope(),
	}

	var dest dummyValue
	err := val.Decode(&dest)
	is.NoErr(err)
	is.Equal(dest, val.Value)
}

func TestAnnotatedEqual(t *testing.T) {
	is := is.New(t)

	val := bass.Annotated{
		Value: dummyValue{
			sentinel: 42,
		},
		Meta: bass.Bindings{
			"doc": bass.String("hello"),
		}.Scope(),
	}

	is.True(val.Equal(val))
	is.True(!val.Equal(bass.Annotated{
		Value: dummyValue{
			sentinel: 43,
		},
		Meta: bass.Bindings{
			"doc": bass.String("hello"),
		}.Scope(),
	}))

	// compare inner value only
	is.True(val.Equal(bass.Annotated{
		Value: dummyValue{
			sentinel: 42,
		},
		Meta: bass.Bindings{
			"doc": bass.String("different"),
		}.Scope(),
	}))
}
