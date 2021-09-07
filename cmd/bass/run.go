package main

import (
	"context"
	"io"
	"os"
	"os/signal"

	"github.com/containerd/console"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/vito/bass"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/prog"
	"github.com/vito/bass/zapctx"
	"golang.org/x/sync/errgroup"
)

func run(ctx context.Context, scope *bass.Scope, filePath string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	recorder := prog.NewRecorder()
	ctx = prog.RecorderToContext(ctx, recorder)

	eg := new(errgroup.Group)

	// start displaying progress so we can initialize the logging vertex
	eg.Go(func() error {
		c, err := console.ConsoleFromFile(os.Stderr)
		if err != nil {
			c = nil
		}

		// don't get interrupted; trust recoder.Close above and exhaust the channel
		progCtx := context.Background()
		return progressui.DisplaySolveStatus(progCtx, "Playing", c, io.Discard, recorder.Source)
	})

	logVertex := recorder.Vertex("log", "[bass]")
	stderr := logVertex.Stderr()

	// wire up logs to vertex
	logger := bass.LoggerTo(stderr)
	ctx = zapctx.ToContext(ctx, logger)

	// wire up stderr for (log), (debug), etc.
	ctx = ioctx.StderrToContext(ctx, stderr)

	// go!
	eg.Go(func() error {
		defer recorder.Close()
		_, err := bass.EvalReader(ctx, scope, file)
		return err
	})

	return eg.Wait()
}
