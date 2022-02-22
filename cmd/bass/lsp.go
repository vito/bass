package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/sourcegraph/jsonrpc2"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/lsp"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/zapctx"
)

func langServer(ctx context.Context) error {
	var logDest io.Writer
	if lspLogs != "" {
		logFile, err := os.Create(lspLogs)
		if err != nil {
			return fmt.Errorf("open lsp log: %w", err)
		}

		defer logFile.Close()

		logDest = logFile
	} else {
		logDest = os.Stderr
	}

	logger := bass.LoggerTo(logDest)

	ctx = zapctx.ToContext(ctx, logger)

	trace := &bass.Trace{}
	ctx = bass.WithTrace(ctx, trace)

	ctx = ioctx.StderrToContext(ctx, logDest)

	pool, err := runtimes.NewPool(&bass.Config{
		// no runtimes; language server must be effect free
		Runtimes: nil,
	})
	if err != nil {
		return fmt.Errorf("new pool: %w", err)
	}

	ctx = bass.WithRuntimePool(ctx, pool)

	logger.Debug("starting")

	<-jsonrpc2.NewConn(
		ctx,
		jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{}),
		lsp.NewHandler(),
	).DisconnectNotify()

	logger.Debug("closed")

	return nil
}

type stdrwc struct{}

func (stdrwc) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

func (c stdrwc) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

func (c stdrwc) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}
