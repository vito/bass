package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/vito/bass"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/lsp"
	"github.com/vito/bass/runtimes"
	"github.com/vito/bass/zapctx"
)

func main() {
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(1)
	}

	logFile, err := os.Create(filepath.Join(os.TempDir(), "bass-lsp.log"))
	if err != nil {
		fmt.Fprintln(os.Stderr, logFile)
		os.Exit(1)
	}

	logger := bass.LoggerTo(logFile)

	ctx := context.Background()
	ctx = zapctx.ToContext(ctx, logger)

	trace := &bass.Trace{}
	ctx = bass.WithTrace(ctx, trace)

	ctx = ioctx.StderrToContext(ctx, logFile)

	pool, err := runtimes.NewPool(&bass.Config{
		// no runtimes; language server must be effect free
		Runtimes: nil,
	})
	if err != nil {
		panic(err)
	}

	ctx = bass.WithRuntime(ctx, pool)

	logger.Debug("starting")

	<-jsonrpc2.NewConn(
		ctx,
		jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{}),
		lsp.NewHandler(),
	).DisconnectNotify()

	logger.Debug("closed")
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
