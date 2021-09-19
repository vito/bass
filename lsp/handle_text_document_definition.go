package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/mattn/go-unicodeclass"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/vito/bass"
	"github.com/vito/bass/std"
	"github.com/vito/bass/zapctx"
	"go.uber.org/zap"
)

func (h *langHandler) handleTextDocumentDefinition(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (result interface{}, err error) {
	if req.Params == nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}

	var params DocumentDefinitionParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	return h.definition(ctx, params.TextDocument.URI, &params)
}

func (h *langHandler) getToken(ctx context.Context, params TextDocumentPositionParams, rest bool) (string, error) {
	logger := zapctx.FromContext(ctx)

	f, ok := h.files[params.TextDocument.URI]
	if !ok {
		return "", fmt.Errorf("document not found: %v", params.TextDocument.URI)
	}

	lines := strings.Split(f.Text, "\n")
	if params.Position.Line < 0 || params.Position.Line > len(lines) {
		return "", fmt.Errorf("invalid position: %v", params.Position)
	}
	chars := utf16.Encode([]rune(lines[params.Position.Line]))
	if params.Position.Character < 0 || params.Position.Character > len(chars) {
		return "", fmt.Errorf("invalid position: %v", params.Position)
	}
	prevPos := 0
	currPos := -1
	prevCls := unicodeclass.Invalid
	for i, char := range chars {
		if i > params.Position.Character && !rest {
			break
		}

		currCls := unicodeclass.Is(rune(char))
		if currCls != prevCls {
			// TODO: use Reader.IsTerminal
			switch char {
			case '_', '-', '?', ':':
				// still a word
				continue
			}

			if i <= params.Position.Character {
				logger.Debug("backward class change", zap.String("char", string(rune(char))))
				prevPos = i
			} else {
				logger.Debug("forward class change", zap.String("char", string(rune(char))))
				currPos = i
				break
			}
		}

		prevCls = currCls
	}
	if currPos == -1 {
		currPos = len(chars)
	}

	return string(utf16.Decode(chars[prevPos:currPos])), nil
}

func (h *langHandler) definition(ctx context.Context, uri DocumentURI, params *DocumentDefinitionParams) ([]Location, error) {
	logger := zapctx.FromContext(ctx)

	word, err := h.getToken(ctx, params.TextDocumentPositionParams, true)
	if err != nil {
		return nil, err
	}

	logger = logger.With(zap.String("tag", word))

	scope, found := h.scopes[uri]
	if !found {
		logger.Warn("scope not initialized", zap.String("uri", string(uri)))
		return nil, nil
	}

	analyzer := h.analyzers[uri]

	var loc bass.Range

	binding := bass.Symbol(word)
	annotated, found := scope.GetWithDoc(binding)
	if found && annotated.Range != (bass.Range{}) {
		logger.Debug("found doc", zap.Any("range", annotated.Range))
		loc = annotated.Range
	} else {
		logger.Debug("no doc; searching lexically", zap.Any("range", annotated.Range))

		loc, found = analyzer.Locate(ctx, binding, params.TextDocumentPositionParams)
	}

	if !found {
		logger.Warn("binding not found")
		return nil, nil
	}

	logger.Info("found definition", zap.Any("loc", loc))

	var defURI DocumentURI

	file := loc.Start.File
	if filepath.IsAbs(file) {
		defURI = toURI(file)
	} else {
		// assume stdlib
		lib, err := std.FS.Open(file)
		if err != nil {
			logger.Warn("not stdlib?")
			return nil, nil
		}

		tmpFile := filepath.Join(os.TempDir(), file)

		tmp, err := os.Create(tmpFile)
		if err != nil {
			logger.Warn("failed to create tmp target file", zap.Error(err))
			return nil, nil
		}

		_, err = io.Copy(tmp, lib)
		if err != nil {
			logger.Warn("failed to write tmp target file", zap.Error(err))
			return nil, nil
		}

		_ = tmp.Close()
		_ = lib.Close()

		defURI = toURI(tmpFile)
	}

	return []Location{
		{
			URI: defURI,
			Range: Range{
				Start: Position{
					Line:      loc.Start.Ln - 1,
					Character: loc.Start.Col,
				},
				End: Position{
					Line:      loc.End.Ln - 1,
					Character: loc.End.Col,
				},
			},
		},
	}, nil
}
