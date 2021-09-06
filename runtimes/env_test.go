package runtimes_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
)

func TestRuntimePlatformDefault(t *testing.T) {
	ctx := context.Background()
	ctx = bass.WithRuntime(ctx, &runtimes.Pool{})

	scope := runtimes.NewScope(bass.Ground, runtimes.RunState{})

	res, err := bass.EvalString(ctx, scope, `(in-image (.cat 42) "alpine")`)
	require.NoError(t, err)
	var wl bass.Workload
	err = res.Decode(&wl)
	require.NoError(t, err)
	require.Equal(t, wl.Platform, bass.LinuxPlatform)

	res, err = bass.EvalString(ctx, scope, `(in-image (on-platform (.cat 42) {:explicit true}) "alpine")`)
	require.NoError(t, err)
	err = res.Decode(&wl)
	require.NoError(t, err)
	require.Equal(t, wl.Platform, bass.Bindings{"explicit": bass.Bool(true)}.Scope())
}
