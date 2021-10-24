package ghcmd_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass/ghcmd"
)

func TestCommandString(t *testing.T) {
	require.Equal(t, "::im::ready", ghcmd.Command{
		Name:  "im",
		Value: "ready",
	}.String())

	require.Equal(t, "::im a=1::since day 1", ghcmd.Command{
		Name:   "im",
		Params: ghcmd.Params{"a": "1"},
		Value:  "since day 1",
	}.String())
}
