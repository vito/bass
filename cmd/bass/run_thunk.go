package main

import (
	"context"
	"os"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/progrock"
)

func runThunk(ctx context.Context) error {
	ctx, pool, err := setupPool(ctx, true)
	if err != nil {
		return err
	}
	defer pool.Close()

	return cli.Step(ctx, cmdline, func(ctx context.Context, vtx *progrock.VertexRecorder) error {
		ctx, runs := bass.TrackRuns(ctx)

		dec := bass.NewRawDecoder(os.Stdin)

		var thunk bass.Thunk
		if err := dec.Decode(&thunk); err != nil {
			return err
		}

		if err := thunk.Run(ctx); err != nil {
			return err
		}

		return runs.Wait()
	})
}
