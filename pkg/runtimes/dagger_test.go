package runtimes_test

import (
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/runtimes"
)

func TestDaggerRuntime(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}

	runtimes.Suite(t, bass.RuntimeConfig{
		Platform: bass.LinuxPlatform,
		Runtime:  runtimes.DaggerName,
	})
}
