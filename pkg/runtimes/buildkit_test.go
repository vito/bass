package runtimes_test

import (
	"context"
	"os"
	"testing"

	"github.com/dagger/testctx"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/runtimes/util/buildkitd"
)

func (RuntimesSuite) TestBuildkit(ctx context.Context, t *testctx.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}

	const testInst = "bass-buildkitd-test"

	_ = buildkitd.Remove(context.Background(), testInst)

	config := bass.Bindings{
		"debug":        bass.Bool(true),
		"installation": bass.String(testInst),
	}

	if dir, ok := os.LookupEnv("BASS_TLS_DEPOT"); ok && dir != "" {
		config["certs_dir"] = bass.String(dir)
	}

	runtimes.Suite(ctx, t, bass.RuntimeConfig{
		Platform: bass.LinuxPlatform,
		Runtime:  runtimes.BuildkitName,
		Config:   config.Scope(),
	})
}
