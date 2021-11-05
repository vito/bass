package ghcmd_test

import (
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass/ghcmd"
)

func TestCommandString(t *testing.T) {
	is := is.New(t)

	is.Equal(ghcmd.Command{
		Name:  "im",
		Value: "ready",
	}.String(), "::im::ready")

	is.Equal(ghcmd.Command{
		Name:   "im",
		Params: ghcmd.Params{"a": "1"},
		Value:  "since day 1",
	}.String(), "::im a=1::since day 1")
}
