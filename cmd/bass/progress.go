package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/adrg/xdg"
	"github.com/vito/bass"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/zapctx"
	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
	"golang.org/x/sync/errgroup"
)

func withProgress(ctx context.Context, f func(context.Context, *progrock.Recorder) error) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	statuses, recorder, err := electRecorder(ctx)
	if err != nil {
		return err
	}

	ctx = progrock.RecorderToContext(ctx, recorder)

	eg := new(errgroup.Group)

	if statuses != nil {
		eg.Go(func() error {
			// start reading progress so we can initialize the logging vertex
			return ui.Display("Playing", os.Stderr, statuses)
		})
	}

	// go!
	eg.Go(func() (err error) {
		defer recorder.Close()

		bassVertex := recorder.Vertex("bass", "[bass]")
		defer func() { bassVertex.Done(err) }()

		stderr := bassVertex.Stderr()

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

func electRecorder(ctx context.Context) (ui.Reader, *progrock.Recorder, error) {
	socketPath, err := xdg.CacheFile(fmt.Sprintf("bass/recorder.%d.sock", syscall.Getpgrp()))
	if err != nil {
		return nil, nil, err
	}

	var r ui.Reader
	var w progrock.Writer
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		r = nil // don't display any progress; send it to the leader
		w, err = progrock.DialRPC("unix", socketPath)
	} else {
		r, w, err = progrock.ServeRPC(l)
	}

	return r, progrock.NewRecorder(w), err
}
