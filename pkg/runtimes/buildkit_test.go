package runtimes_test

import (
	"context"
	"testing"

	_ "github.com/moby/buildkit/client"
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

	ctx := context.Background()

	pool, err := runtimes.NewPool(ctx, &bass.Config{
		Runtimes: []bass.RuntimeConfig{
			{
				Platform: bass.LinuxPlatform,
				Runtime:  runtimes.BuildkitName,
				Config: bass.Bindings{
					"certs_dir": bass.String("./testdata/tls/"),
				}.Scope(),
			},
		},
	})
	is.NoErr(err)

	runtimes.Suite(t, pool)
}
