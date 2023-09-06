package main

import (
	"context"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
)

func repl(ctx context.Context) error {
	ctx, pool, err := setupPool(ctx, false)
	if err != nil {
		return err
	}
	defer pool.Close()

	scope := bass.NewRunScope(bass.Ground, bass.RunState{
		Dir:    bass.NewHostDir("."),
		Stdin:  bass.Stdin,
		Stdout: bass.Stdout,
		Env:    bass.ImportSystemEnv(),
	})

	return cli.Repl(ctx, scope)
}
