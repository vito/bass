package docker_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/bass/runtimes/docker"
)

func TestDocker(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}

	pool := &runtimes.Pool{}

	tmp := filepath.Join(os.TempDir(), "bass-tests")
	if testing.CoverMode() != "" {
		// with -cover, run with a clean slate so caches aren't hit
		var err error
		tmp, err = os.MkdirTemp("", "bass-tests")
		require.NoError(t, err)
	}

	// TODO: cleaning up the data dir is currently impossible as it requires root
	// permissions. :(

	runtime, err := docker.NewRuntime(pool, bass.Object{
		"data": bass.String(tmp),
	})
	require.NoError(t, err)

	pool.Runtimes = append(pool.Runtimes, runtimes.Assoc{
		Platform: runtimes.TestPlatform,
		Runtime:  runtime,
	})

	runtimes.Suite(t, pool)
}
