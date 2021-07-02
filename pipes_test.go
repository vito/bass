package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestSinkDecode(t *testing.T) {
	sink := &bass.JSONSink{}
	val := &bass.Sink{sink}

	var res bass.PipeSink
	err := val.Decode(&res)
	require.NoError(t, err)
	require.Equal(t, sink, res)

	var same *bass.Sink
	err = val.Decode(&same)
	require.NoError(t, err)
	require.Equal(t, val, same)
}

func TestSinkEqual(t *testing.T) {
	sink := &bass.JSONSink{}
	val := &bass.Sink{sink}

	require.True(t, val.Equal(val))
	require.False(t, val.Equal(&bass.Sink{sink}))
}

func TestSourceDecode(t *testing.T) {
	sink := &bass.JSONSource{}
	val := &bass.Source{sink}

	var res bass.PipeSource
	err := val.Decode(&res)
	require.NoError(t, err)
	require.Equal(t, sink, res)

	var same *bass.Source
	err = val.Decode(&same)
	require.NoError(t, err)
	require.Equal(t, val, same)
}

func TestSourceEqual(t *testing.T) {
	sink := &bass.JSONSource{}
	val := &bass.Source{sink}

	require.True(t, val.Equal(val))
	require.False(t, val.Equal(&bass.Source{sink}))
}
