package runtimes_test

import (
	"os"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/is"
)

func TestBuildkitRuntime(t *testing.T) {
	is := is.New(t)

	if testing.Short() {
		t.SkipNow()
		return
	}

	is.NoErr(os.Chmod("./testdata/tls/bass.crt", 0400))
	is.NoErr(os.Chmod("./testdata/tls/bass.key", 0400))

	runtimes.Suite(t, bass.RuntimeConfig{
		Platform: bass.LinuxPlatform,
		Runtime:  runtimes.BuildkitName,
		Config: bass.Bindings{
			"debug":     bass.Bool(true),
			"certs_dir": bass.String("./testdata/tls/"),
		}.Scope(),
	})
}
