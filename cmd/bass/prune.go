package main

import (
	"context"
	"fmt"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/progrock"
)

func prune(ctx context.Context) error {
	ctx, _, err := setupPool(ctx, true)
	if err != nil {
		return err
	}

	return cli.Step(ctx, cmdline, func(ctx context.Context, vertex *progrock.VertexRecorder) error {
		pool, err := bass.RuntimePoolFromContext(ctx)
		if err != nil {
			return err
		}

		runtimes, err := pool.All()
		if err != nil {
			return err
		}

		for i, runtime := range runtimes {
			err := runtime.Prune(ctx, bass.PruneOpts{})
			if err != nil {
				return fmt.Errorf("prune runtime #%d: %w", i+1, err)
			}
		}

		return nil
	})
}
