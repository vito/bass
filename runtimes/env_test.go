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

	env := runtimes.NewEnv(bass.Ground, runtimes.RunState{})

	res, err := bass.EvalString(ctx, env, `(in-image (.cat 42) "alpine")`)
	require.NoError(t, err)
	var wl bass.Workload
	err = res.Decode(&wl)
	require.NoError(t, err)
	require.Equal(t, wl.Platform, bass.LinuxPlatform)

	res, err = bass.EvalString(ctx, env, `(in-image (on-platform (.cat 42) {:explicit true}) "alpine")`)
	require.NoError(t, err)
	err = res.Decode(&wl)
	require.NoError(t, err)
	require.Equal(t, wl.Platform, bass.Object{"explicit": bass.Bool(true)})
}
