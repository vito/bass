package bass_test

import (
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
)

func TestSinkDecode(t *testing.T) {
	is := is.New(t)

	sink := &bass.JSONSink{}
	val := &bass.Sink{sink}

	var res bass.PipeSink
	err := val.Decode(&res)
	is.NoErr(err)
	is.Equal(res, sink)

	var same *bass.Sink
	err = val.Decode(&same)
	is.NoErr(err)
	is.Equal(same, val)
}

func TestSinkEqual(t *testing.T) {
	is := is.New(t)

	sink := &bass.JSONSink{}
	val := &bass.Sink{sink}

	is.True(val.Equal(val))
	is.True(!val.Equal(&bass.Sink{sink}))
}

func TestSourceDecode(t *testing.T) {
	is := is.New(t)

	sink := &bass.JSONSource{}
	val := &bass.Source{sink}

	var res bass.PipeSource
	err := val.Decode(&res)
	is.NoErr(err)
	is.Equal(res, sink)

	var same *bass.Source
	err = val.Decode(&same)
	is.NoErr(err)
	is.Equal(same, val)
}

func TestSourceEqual(t *testing.T) {
	is := is.New(t)

	sink := &bass.JSONSource{}
	val := &bass.Source{sink}

	is.True(val.Equal(val))
	is.True(!val.Equal(&bass.Source{sink}))
}
