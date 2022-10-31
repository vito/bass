package runtimes_test

import (
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstls"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/is"
)

func TestBuildkitRuntime(t *testing.T) {
	is := is.New(t)

	if testing.Short() {
		t.SkipNow()
		return
	}

	tls := t.TempDir()
	is.NoErr(basstls.Init(tls))

	runtimes.Suite(t, bass.RuntimeConfig{
		Platform: bass.LinuxPlatform,
		Runtime:  runtimes.BuildkitName,
		Config: bass.Bindings{
			"debug":        bass.Bool(true),
			"installation": bass.String("bass-buildkitd-test"),
			"certs_dir":    bass.String(tls),
		}.Scope(),
	})
}
