package runtimes_test

import (
	"os"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/runtimes"
)

func TestDaggerRuntime(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}

	daggerHost := os.Getenv("DAGGER_HOST")
	if daggerHost == "" {
		t.Skipf("$DAGGER_HOST not set; skipping!")
		return
	}

	runtimes.Suite(t, bass.RuntimeConfig{
		Platform: bass.LinuxPlatform,
		Runtime:  runtimes.DaggerName,
	})
}
