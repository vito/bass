package lsp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

func (h *langHandler) handleTextDocumentFormatting(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (result any, err error) {
	if req.Params == nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}

	var params DocumentFormattingParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	return h.formatRequest(params.TextDocument.URI, params.Options)
}

func (h *langHandler) formatRequest(uri DocumentURI, opt FormattingOptions) ([]TextEdit, error) {
	return nil, fmt.Errorf("not implemented")
}
