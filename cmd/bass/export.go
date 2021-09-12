package main

import (
	"context"
	"os"

	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/progrock"
)

var runExport bool

func init() {
	rootCmd.Flags().BoolVarP(&runExport, "export", "e", false, "write a workload path to stdout (directories are in tar format)")
}

func export(ctx context.Context, pool *runtimes.Pool) error {
	dec := bass.NewDecoder(os.Stdin)

	var path bass.WorkloadPath
	err := dec.Decode(&path)
	if err != nil {
		bass.WriteError(ctx, Stderr, err)
		return err
	}

	return withProgress(ctx, func(ctx context.Context, recorder *progrock.Recorder) error {
		return pool.Export(ctx, os.Stdout, path.Workload, path.Path.FilesystemPath())
	})
}
