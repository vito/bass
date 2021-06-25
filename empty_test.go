package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestEmptyDecode(t *testing.T) {
	var dest bass.List
	err := bass.Empty{}.Decode(&dest)
	require.NoError(t, err)
	require.Equal(t, bass.Empty{}, dest)
}

func TestEmptyEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Empty{}

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, val, res)
}

func TestEmptyListInterface(t *testing.T) {
	var list bass.List = bass.Empty{}
	require.Equal(t, list.First(), bass.Empty{})
	require.Equal(t, list.Rest(), bass.Empty{})
}
