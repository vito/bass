package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/vito/bass"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/zapctx"
	"github.com/vito/progrock"
	"golang.org/x/sync/errgroup"
)

func withProgress(ctx context.Context, f func(context.Context, *progrock.Recorder) error) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	recorder := progrock.NewRecorder()
	ctx = progrock.RecorderToContext(ctx, recorder)

	eg := new(errgroup.Group)

	eg.Go(func() error {
		// start reading progress so we can initialize the logging vertex
		return recorder.Display("Playing", os.Stderr)
	})

	// go!
	eg.Go(func() (err error) {
		defer recorder.Close()

		logVertex := recorder.Vertex("log", "[bass]")
		defer func() { logVertex.Done(err) }()

		stderr := logVertex.Stderr()

		// wire up logs to vertex
		logger := bass.LoggerTo(stderr)
		ctx = zapctx.ToContext(ctx, logger)

		// wire up stderr for (log), (debug), etc.
		ctx = ioctx.StderrToContext(ctx, stderr)

		err = f(ctx, recorder)
		if err != nil {
			bass.WriteError(ctx, err)
			return err
		}

		return
	})

	return eg.Wait()
}
