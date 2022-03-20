package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/morikuni/aec"
	"github.com/opencontainers/go-digest"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
)

var fancy bool

func init() {
	fancy = os.Getenv("BASS_FANCY_TUI") != ""
}

var UI = ui.Default

func init() {
	UI.ConsoleRunning = "Playing %s (%d/%d)"
	UI.ConsoleDone = "Playing %s (%d/%d) " + aec.GreenF.Apply("done")
}

func withProgress(ctx context.Context, name string, f func(context.Context, *progrock.VertexRecorder) error) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	statuses, recorder, err := electRecorder()
	if err != nil {
		bass.WriteError(ctx, err)
		return err
	}

	defer recorder.Stop()

	ctx = progrock.RecorderToContext(ctx, recorder)

	if statuses != nil {
		defer cleanupRecorder()

		recorder.Display(stop, UI, os.Stderr, statuses, fancy)
	}

	bassVertex := recorder.Vertex(digest.Digest(name), fmt.Sprintf("bass %s", name))
	defer func() { bassVertex.Done(err) }()

	stderr := bassVertex.Stderr()

	// wire up logs to vertex
	logger := bass.LoggerTo(stderr)
	ctx = zapctx.ToContext(ctx, logger)

	// wire up stderr for (log), (debug), etc.
	ctx = ioctx.StderrToContext(ctx, stderr)

	err = f(ctx, bassVertex)
	if err != nil {
		bass.WriteError(ctx, err)
		return err
	}

	return nil
}
