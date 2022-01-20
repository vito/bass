package main

import (
	"context"
	"fmt"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/progrock"
)

var runPrune bool

func init() {
	rootCmd.Flags().BoolVar(&runPrune, "prune", false, "release data and caches retained by runtimes")
}

func prune(ctx context.Context) error {
	return withProgress(ctx, "prune", func(ctx context.Context, vertex *progrock.VertexRecorder) error {
		pool, err := bass.RuntimePoolFromContext(ctx)
		if err != nil {
			return err
		}

		for i, runtime := range pool.All() {
			err := runtime.Prune(ctx, bass.PruneOpts{})
			if err != nil {
				return fmt.Errorf("prune runtime #%d: %w", i+1, err)
			}
		}

		return nil
	})
}
