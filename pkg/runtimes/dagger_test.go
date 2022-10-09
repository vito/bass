package runtimes_test

import (
	"context"
	"os"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/runtimes/util/buildkitd"
	"github.com/vito/is"
)

func TestDaggerRuntime(t *testing.T) {
	is := is.New(t)

	if testing.Short() {
		t.SkipNow()
		return
	}

	host, err := buildkitd.Start(context.Background())
	is.NoErr(err)

	os.Setenv("BUILDKIT_HOST", host)

	ctx := context.Background()

	pool, err := runtimes.NewPool(ctx, &bass.Config{
		Runtimes: []bass.RuntimeConfig{
			{
				Platform: bass.LinuxPlatform,
				Runtime:  runtimes.DaggerName,
			},
		},
	})
	is.NoErr(err)

	runtimes.Suite(t, pool)
}
