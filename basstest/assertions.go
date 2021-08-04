package basstest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func Equal(t *testing.T, a, b bass.Value) {
	require.True(t, a.Equal(b), "%s != %s", a, b)
}
