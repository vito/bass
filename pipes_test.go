package bass_test

import (
	"testing"

	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/is"
)

func TestSinkDecode(t *testing.T) {
	is := is.New(t)

	sink := &bass.JSONSink{}
	val := &bass.Sink{sink}

	var res bass.PipeSink
	err := val.Decode(&res)
	is.NoErr(err)
	is.True(res == sink)

	var same *bass.Sink
	err = val.Decode(&same)
	is.NoErr(err)
	is.True(same == val)
}

func TestSinkEqual(t *testing.T) {
	is := is.New(t)

	sink := &bass.JSONSink{}
	val := &bass.Sink{sink}

	Equal(t, val, val)
	is.True(!val.Equal(&bass.Sink{sink}))
}

func TestSourceDecode(t *testing.T) {
	is := is.New(t)

	sink := &bass.JSONSource{}
	val := &bass.Source{sink}

	var res bass.PipeSource
	err := val.Decode(&res)
	is.NoErr(err)
	is.True(res == sink)

	var same *bass.Source
	err = val.Decode(&same)
	is.NoErr(err)
	is.True(same == val)
}

func TestSourceEqual(t *testing.T) {
	is := is.New(t)

	sink := &bass.JSONSource{}
	val := &bass.Source{sink}

	Equal(t, val, val)
	is.True(!val.Equal(&bass.Source{sink}))
}
