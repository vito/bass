package runtimes_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
)

func TestDockerRuntime(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}

	tmp := filepath.Join(os.TempDir(), "bass-tests")
	if testing.CoverMode() != "" {
		// with -cover, run with a clean slate so caches aren't hit
		var err error
		tmp, err = os.MkdirTemp("", "bass-tests")
		require.NoError(t, err)
	}

	pool, err := runtimes.NewPool(&bass.Config{
		Runtimes: []bass.RuntimeConfig{
			{
				Platform: bass.LinuxPlatform,
				Runtime:  runtimes.DockerName,
				Config: bass.Object{
					"data": bass.String(tmp),
				},
			},
		},
	})
	require.NoError(t, err)

	// TODO: cleaning up the data dir is currently impossible as it requires root
	// permissions. :(

	runtimes.Suite(t, pool)
}
