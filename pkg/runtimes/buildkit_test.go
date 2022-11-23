package runtimes_test

import (
	"context"
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

	const testInst = "bass-buildkitd-test"

	buildkitd.Remove(context.Background(), testInst)

	runtimes.Suite(t, bass.RuntimeConfig{
		Platform: bass.LinuxPlatform,
		Runtime:  runtimes.BuildkitName,
		Config: bass.Bindings{
			"debug":        bass.Bool(true),
			"installation": bass.String(testInst),
		}.Scope(),
	})
}
