package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/mattn/go-isatty"
	"github.com/vito/bass/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/progrock"
)

func run(ctx context.Context, filePath string, argv ...string) error {
	args := []bass.Value{}
	for _, arg := range argv {
		args = append(args, bass.String(arg))
	}

	analogousThunk := bass.Thunk{
		Cmd: bass.ThunkCmd{
			Host: &bass.HostPath{
				ContextDir: filepath.Dir(filePath),
				Path: bass.FileOrDirPath{
					File: &bass.FilePath{Path: filepath.Base(filePath)},
				},
			},
		},
		Args: args,
	}

	return withProgress(ctx, analogousThunk.Cmdline(), func(ctx context.Context, bassVertex *progrock.VertexRecorder) error {
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}

		defer file.Close()

		isTerm := isatty.IsTerminal(os.Stdout.Fd())

		stdout := bass.Stdout
		if isTerm {
			stdout = bass.NewSink(bass.NewJSONSink("stdout vertex", bassVertex.Stdout()))
		}

		env := bass.ImportSystemEnv()

		scope := runtimes.NewScope(bass.Ground, runtimes.RunState{
			Dir:    bass.NewHostPath(filepath.Dir(filePath) + string(os.PathSeparator)),
			Args:   bass.NewList(args...),
			Stdin:  bass.Stdin,
			Stdout: stdout,
			Env:    env,
		})

		_, err = bass.EvalReader(ctx, scope, file)
		if err != nil && !isTerm {
			os.Stdout.Close()
		}

		return err
	})
}
