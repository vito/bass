package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/mattn/go-isatty"
	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/progrock"
)

func run(ctx context.Context, filePath string, argv ...string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	args := []bass.Value{}
	for _, arg := range argv {
		args = append(args, bass.String(arg))
	}

	return withProgress(ctx, func(ctx context.Context, recorder *progrock.Recorder) error {
		bassVertex := recorder.Vertex("bass", "[bass]")
		defer func() { bassVertex.Done(err) }()

		stdout := bass.Stdout
		if isatty.IsTerminal(os.Stdout.Fd()) {
			stdout = bass.NewSink(bass.NewJSONSink("stdout vertex", bassVertex.Stdout()))
		}

		scope := runtimes.NewScope(bass.Ground, runtimes.RunState{
			Dir:    bass.HostPath{Path: filepath.Dir(filePath)},
			Args:   bass.NewList(args...),
			Stdin:  bass.Stdin,
			Stdout: stdout,
		})

		_, err := bass.EvalReader(ctx, scope, file)
		return err
	})
}
