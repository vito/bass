package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattn/go-isatty"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
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

	return withProgress(ctx, analogousThunk.Cmdline(), func(ctx context.Context, bassVertex *progrock.VertexRecorder) (err error) {
		isTerm := isatty.IsTerminal(os.Stdout.Fd())

		if !isTerm {
			defer func() {
				// ensure a chained unix pipeline exits
				if err != nil && !isTerm {
					os.Stdout.Close()
				}
			}()
		}

		stdout := bass.Stdout
		if isTerm {
			stdout = bass.NewSink(bass.NewJSONSink("stdout vertex", bassVertex.Stdout()))
		}

		env := bass.ImportSystemEnv()

		stdin := bass.Stdin
		if len(inputs) > 0 {
			stdin, err = cli.InputsSource(inputs)
			if err != nil {
				err = fmt.Errorf("inputs: %w", err)
				return
			}
		}

		scope := bass.NewRunScope(bass.Ground, bass.RunState{
			Dir:    bass.NewHostDir(filepath.Dir(filePath) + string(os.PathSeparator)),
			Stdin:  stdin,
			Stdout: stdout,
			Env:    env,
		})

		_, err = bass.EvalFile(ctx, scope, filePath)
		if err != nil {
			return
		}

		err = bass.RunMain(ctx, scope, args...)
		return
	})
}
