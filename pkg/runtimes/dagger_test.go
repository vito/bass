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

	if os.Getenv("RUN_DAGGER_TESTS") == "" {
		t.Skipf("$RUN_DAGGER_TESTS not set; skipping!")
		return
	}

	runtimes.Suite(t, bass.RuntimeConfig{
		Platform: bass.LinuxPlatform,
		Runtime:  runtimes.DaggerName,
	})
}
