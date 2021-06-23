package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestApplyInterface(t *testing.T) {
	var list bass.Apply = bass.Empty{}
	require.Equal(t, list.First(), bass.Empty{})
	require.Equal(t, list.Rest(), bass.Empty{})

	_ = list.(bass.List)

	list = bass.Pair{bass.Int(1), bass.Bool(true)}
	require.Equal(t, list.First(), bass.Int(1))
	require.Equal(t, list.Rest(), bass.Bool(true))
}
