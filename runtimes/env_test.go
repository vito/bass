package runtimes_test

import (
	"context"
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
)

func TestRuntimePlatformDefault(t *testing.T) {
	is := is.New(t)

	ctx := context.Background()
	ctx = bass.WithRuntime(ctx, &runtimes.Pool{})

	scope := runtimes.NewScope(bass.Ground, runtimes.RunState{})

	res, err := bass.EvalString(ctx, scope, `(in-image (.cat 42) "alpine")`)
	is.NoErr(err)
	var wl bass.Workload
	err = res.Decode(&wl)
	is.NoErr(err)
	is.Equal(bass.LinuxPlatform, wl.Platform)

	res, err = bass.EvalString(ctx, scope, `(in-image (on-platform (.cat 42) {:explicit true}) "alpine")`)
	is.NoErr(err)
	err = res.Decode(&wl)
	is.NoErr(err)
	is.Equal(bass.Bindings{"explicit": bass.Bool(true)}.Scope(), wl.Platform)
}
