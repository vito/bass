package main

import (
	"context"
	"os"

	"github.com/vito/bass"
	"github.com/vito/progrock"
)

func run(ctx context.Context, scope *bass.Scope, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	return withProgress(ctx, func(ctx context.Context, recorder *progrock.Recorder) error {
		_, err := bass.EvalReader(ctx, scope, file)
		return err
	})
}
