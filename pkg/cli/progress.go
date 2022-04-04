package cli

import (
	"github.com/morikuni/aec"
	"github.com/vito/progrock/ui"
)

var ProgressUI = ui.Default

func init() {
	ProgressUI.ConsoleRunning = "Playing %s (%d/%d)"
	ProgressUI.ConsoleDone = "Playing %s (%d/%d) " + aec.GreenF.Apply("done")
}
