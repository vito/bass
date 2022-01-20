package lsp

import (
	"context"
	"encoding/json"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/vito/bass/bass"
	"github.com/vito/bass/zapctx"
	"go.uber.org/zap"
)

func (h *langHandler) handleTextDocumentCompletion(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (result interface{}, err error) {
	if req.Params == nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}

	var params CompletionParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	return h.completion(ctx, params.TextDocument.URI, &params)
}

func (h *langHandler) completion(ctx context.Context, uri DocumentURI, params *CompletionParams) ([]CompletionItem, error) {
	logger := zapctx.FromContext(ctx)

	prefix, err := h.getToken(ctx, params.TextDocumentPositionParams, false)
	if err != nil {
		logger.Error("failed to get token", zap.Error(err))
		return nil, err
	}

	logger = logger.With(zap.String("prefix", prefix))

	scope, found := h.scopes[uri]
	if !found {
		logger.Warn("scope not initialized", zap.String("uri", string(uri)))
		return nil, nil
	}

	analyzer := h.analyzers[uri]

	logger.Debug("completing")

	suggested := map[bass.Symbol]bool{}
	var items []CompletionItem
	for _, opt := range scope.Complete(prefix) {
		var kind CompletionItemKind = VariableCompletion

		var app bass.Applicative
		if opt.Value.Decode(&app) == nil {
			kind = FunctionCompletion
		}

		var op *bass.Operative
		if opt.Value.Decode(&op) == nil {
			// XXX: not sure if this is appropriate
			kind = OperatorCompletion
		}

		label := opt.Binding.String()

		logger.Debug("suggesting", zap.String("label", label))

		suggested[opt.Binding] = true

		var doc string
		if val, found := opt.Value.Meta.Get("doc"); found {
			if err := val.Decode(&doc); err != nil {
				logger.Sugar().Warnf("doc value must be a string, but have %T", val)
			}
		}

		items = append(items, CompletionItem{
			Label:         label,
			Kind:          kind, // XXX: ?
			Detail:        bass.Details(opt.Value.Value),
			Documentation: doc,
		})
	}

	for _, binding := range analyzer.Complete(ctx, prefix, params.TextDocumentPositionParams) {
		if suggested[binding] {
			continue
		}

		items = append(items, CompletionItem{
			Label:  binding.String(),
			Kind:   VariableCompletion,
			Detail: "lexical binding",
		})
	}

	return items, nil
}
