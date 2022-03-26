package lsp

import (
	"context"

	"github.com/sourcegraph/jsonrpc2"
)

func (h *langHandler) handleShutdown(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (result any, err error) {
	return nil, conn.Close()
}
