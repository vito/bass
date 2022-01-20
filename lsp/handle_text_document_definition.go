package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/vito/bass/bass"
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

	start := 0
	end := -1
	for i, char := range chars {
		if i > params.Position.Character && !rest {
			break
		}

		if isTerminal(rune(char)) {
			if i < params.Position.Character {
				start = i + 1
			} else {
				end = i
				break
			}
		}
	}
	if end == -1 {
		end = len(chars)
	}

	return string(utf16.Decode(chars[start:end])), nil
}

var terminal = map[rune]bool{
	'"': true,
	'(': true,
	')': true,
	'[': true,
	']': true,
	'{': true,
	'}': true,
	';': true,
}

// XXX: mirrors bass.Reader.IsTerminal
func isTerminal(r rune) bool {
	if isSpace(r) {
		return true
	}

	return terminal[r]
}

func isSpace(r rune) bool {
	return unicode.IsSpace(r) || r == ','
}

func rangeFromMeta(meta *bass.Scope) (bass.Range, error) {
	var r bass.Range
	if err := meta.GetDecode("file", &r.Start.File); err != nil {
		return r, err
	}

	if err := meta.GetDecode("line", &r.Start.Ln); err != nil {
		return r, err
	}

	if err := meta.GetDecode("column", &r.Start.Col); err != nil {
		return r, err
	}

	r.End = r.Start

	return r, nil
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

	binding := bass.Symbol(word)

	loc, found := analyzer.Locate(ctx, binding, params.TextDocumentPositionParams)
	if found {
		logger.Debug("found definition lexically", zap.Any("range", loc))
	} else if val, found := scope.Get(binding); found {
		var annotated bass.Annotated
		if err := val.Decode(&annotated); err != nil {
			logger.Warn("binding has no annotation")
			return nil, nil
		}

		var err error
		loc, err = rangeFromMeta(annotated.Meta)
		if err != nil {
			logger.Error("no range in meta", zap.Error(err), zap.Any("meta", annotated.Meta), zap.Any("value", annotated.Value))
			return nil, err
		}

		logger.Debug("found definitian via doc", zap.Any("range", loc))
	} else {
		logger.Warn("definition not found")
		return nil, nil
	}

	var defURI DocumentURI

	file := loc.Start.File
	if filepath.IsAbs(file) {
		defURI = toURI(file)
	} else {
		// assume stdlib
		lib, err := std.FS.Open(file)
		if err != nil {
			logger.Warn("not stdlib?", zap.String("file", file))
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
