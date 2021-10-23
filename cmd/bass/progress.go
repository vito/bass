package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/adrg/xdg"
	"github.com/containerd/console"
	"github.com/morikuni/aec"
	"github.com/opencontainers/go-digest"
	"github.com/vito/bass"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/zapctx"
	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
)

var simpleProgress bool

func init() {
	simpleProgress = os.Getenv("BASS_SIMPLE_OUTPUT") != ""
}

var UI = ui.Default

func init() {
	UI.ConsoleRunning = "Playing %.1fs (%d/%d)"
	UI.ConsoleDone = "Playing %.1fs (%d/%d) " + aec.GreenF.Apply("done")
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
		var c console.Console
		if !simpleProgress {
			c = console.Current()
		}

		// distinct ctx to prevent stopping progress when handling interrupt
		recorder.Display(context.Background(), UI, c, os.Stderr, statuses)
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

func electRecorder() (ui.Reader, *progrock.Recorder, error) {
	socketPath, err := xdg.StateFile(fmt.Sprintf("bass/recorder.%d.sock", syscall.Getpgrp()))
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
