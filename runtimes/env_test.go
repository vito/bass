package runtimes_test

import (
	"context"
	"testing"

	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/is"
)

func TestRuntimePlatformDefault(t *testing.T) {
	is := is.New(t)

	ctx := context.Background()
	ctx = runtimes.WithPool(ctx, &runtimes.Pool{})

	scope := runtimes.NewScope(bass.Ground, runtimes.RunState{
		Dir:    bass.HostPath{Path: bass.ParseFileOrDirPath(".")},
		Args:   bass.Empty{},
		Stdin:  bass.NewSource(bass.NewInMemorySource()),
		Stdout: bass.NewSink(bass.NewInMemorySink()),
	})

	var thunk bass.Thunk

	res, err := bass.EvalString(ctx, scope, `
		(.cat 42)
	`)
	is.NoErr(err)
	err = res.Decode(&thunk)
	is.NoErr(err)
	is.Equal(nil, thunk.Platform())

	res, err = bass.EvalString(ctx, scope, `
		(in-image (.cat 42) "alpine")
	`)
	is.NoErr(err)
	err = res.Decode(&thunk)
	is.NoErr(err)
	is.Equal(&bass.LinuxPlatform, thunk.Platform())

	res, err = bass.EvalString(ctx, scope, `
		(in-image (.cat 42) {:platform {:os "explicit"} :repository "alpine"})
	`)
	is.NoErr(err)
	err = res.Decode(&thunk)
	is.NoErr(err)
	is.Equal(&bass.Platform{OS: "explicit"}, thunk.Platform())
}
