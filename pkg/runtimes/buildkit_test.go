package runtimes_test

import (
	"context"
	"os"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/runtimes/util/buildkitd"
)

func TestBuildkitRuntime(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}

	t.Parallel()

	const testInst = "bass-buildkitd-test"

	_ = buildkitd.Remove(context.Background(), testInst)

	config := bass.Bindings{
		"debug":        bass.Bool(true),
		"installation": bass.String(testInst),
	}

	if dir, ok := os.LookupEnv("BASS_TLS_DEPOT"); ok && dir != "" {
		config["certs_dir"] = bass.String(dir)
	}

	runtimes.Suite(testCtx, t, bass.RuntimeConfig{
		Platform: bass.LinuxPlatform,
		Runtime:  runtimes.BuildkitName,
		Config:   config.Scope(),
	})
}
