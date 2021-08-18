package main

import (
	"context"
	"os"

	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
)

var runExport *bool

func init() {
	runExport = rootCmd.Flags().BoolP("export", "e", false, "write a workload path to stdout (directories are in tar format)")
}

func export(ctx context.Context, pool *runtimes.Pool) error {
	dec := bass.NewDecoder(os.Stdin)

	var path bass.WorkloadPath
	err := dec.Decode(&path)
	if err != nil {
		return err
	}

	return pool.Export(ctx, os.Stdout, path.Workload, path.Path.FilesystemPath())
}
