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

	if os.Getenv("SKIP_DAGGER_TESTS") != "" {
		t.Skipf("$SKIP_DAGGER_TESTS set; skipping!")
		return
	}

	runtimes.Suite(t, bass.RuntimeConfig{
		Platform: bass.LinuxPlatform,
		Runtime:  runtimes.DaggerName,
	}, runtimes.SkipSuites(
		"tls.bass",
		"addrs.bass",
		"docker-build.bass",
		"cache-cmd.bass",
		"oci-archive-image.bass",
	// secrets don't get sent over gRPC
	))
}
