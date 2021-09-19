package lsp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

func (h *langHandler) handleTextDocumentCodeAction(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (result interface{}, err error) {
	if req.Params == nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}

	var params CodeActionParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	return h.codeAction(params.TextDocument.URI, &params)
}

func (h *langHandler) executeCommand(params *ExecuteCommandParams) (interface{}, error) {
	// TODO
	return nil, fmt.Errorf("not implemented")
}

func (h *langHandler) codeAction(uri DocumentURI, params *CodeActionParams) ([]Command, error) {
	// TODO
	return []Command{}, nil
}
