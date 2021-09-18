package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/vito/bass"
	"github.com/vito/bass/lsp"
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

	logger.Debug("started")

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
