package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestIgnoreDecode(t *testing.T) {
	ign := bass.Ignore{}
	err := bass.Ignore{}.Decode(&ign)
	require.NoError(t, err)
	require.Equal(t, ign, bass.Ignore{})

	str := "some untouched value"
	err = bass.Ignore{}.Decode(&str)
	require.Equal(t, bass.DecodeError{
		Source:      bass.Ignore{},
		Destination: &str,
	}, err)
}

func TestIgnoreEqual(t *testing.T) {
	require.True(t, bass.Ignore{}.Equal(bass.Ignore{}))
	require.True(t, bass.Ignore{}.Equal(wrappedValue{bass.Ignore{}}))
	require.False(t, bass.Ignore{}.Equal(bass.Null{}))
}

func TestIgnoreEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Ignore{}

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, val, res)
}
