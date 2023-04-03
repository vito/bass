package cli

import (
	"context"
	"path/filepath"

	"github.com/vito/bass/pkg/bass"
)

func Run(ctx context.Context, env *bass.Scope, inputs []string, filePath string, argv []string, stdout *bass.Sink) error {
	ctx, runs := bass.TrackRuns(ctx)

	dir, base := filepath.Split(filePath)

	cmd := bass.NewHostPath(
		dir,
		bass.ParseFileOrDirPath(filepath.ToSlash(base)),
	)

	thunk := bass.Thunk{
		Args: []bass.Value{cmd},
		Env:  env,
	}

	for _, arg := range argv {
		thunk.Args = append(thunk.Args, bass.String(arg))
	}

	stdin := bass.Stdin
	if len(inputs) > 0 {
		stdin = InputsSource(inputs)
	}

	err := bass.NewBass().Run(ctx, thunk, bass.RunState{
		Dir:    bass.NewHostDir(filepath.Dir(filePath)),
		Stdin:  stdin,
		Stdout: stdout,
		Env:    thunk.Env,
	})
	if err != nil {
		return err
	}

	return runs.StopAndWait()
}
