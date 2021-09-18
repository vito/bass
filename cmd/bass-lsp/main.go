package main

import (
	"context"
	"flag"
	"log"
	"os"

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

	logger := bass.Logger()

	ctx := context.Background()
	ctx = zapctx.ToContext(ctx, logger)

	log.Println("bass-lsp: reading on stdin, writing on stdout")

	<-jsonrpc2.NewConn(
		ctx,
		jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{}),
		lsp.NewHandler(),
	).DisconnectNotify()

	log.Println("bass-lsp: connections closed")
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
