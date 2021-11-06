package runtimes_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/is"
)

func TestDockerRuntime(t *testing.T) {
	is := is.New(t)

	if testing.Short() {
		t.SkipNow()
		return
	}

	tmp := filepath.Join(os.TempDir(), "bass-tests")
	if testing.CoverMode() != "" {
		// with -cover, run with a clean slate so caches aren't hit
		var err error
		tmp, err = os.MkdirTemp("", "bass-tests")
		is.NoErr(err)
	}

	pool, err := runtimes.NewPool(&bass.Config{
		Runtimes: []bass.RuntimeConfig{
			{
				Platform: bass.LinuxPlatform,
				Runtime:  runtimes.DockerName,
				Config: bass.Bindings{
					"data": bass.String(tmp)}.Scope(),
			},
		},
	})
	is.NoErr(err)

	// TODO: cleaning up the data dir is currently impossible as it requires root
	// permissions. :(

	runtimes.Suite(t, pool)
}
