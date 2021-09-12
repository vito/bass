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

func run(ctx context.Context, scope *bass.Scope, filePath string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

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

		_, err = bass.EvalReader(ctx, scope, file)
		if err != nil {
			bass.WriteError(ctx, stderr, err)
			return err
		}

		return nil
	})

	return eg.Wait()
}
