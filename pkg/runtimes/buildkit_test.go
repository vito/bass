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

	// coordinate with bass/buildkit.bass test CNI config
	const testGatewayIP = "10.73.0.1"

	pool, err := runtimes.NewPool(ctx, &bass.Config{
		Runtimes: []bass.RuntimeConfig{
			{
				Platform: bass.LinuxPlatform,
				Runtime:  runtimes.BuildkitName,
				Config: bass.Bindings{
					"gateway_ip": bass.String(testGatewayIP),
				}.Scope(),
			},
		},
	})
	is.NoErr(err)

	runtimes.Suite(t, pool)
}
