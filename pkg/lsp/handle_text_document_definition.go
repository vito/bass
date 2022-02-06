package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/adrg/xdg"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/vito/bass/pkg"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/bass/std"
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
		if _, err := os.Stat(file); err != nil {
			logger.Error("cannot stat definition file", zap.Error(err))
			return nil, err
		}

		defURI = toURI(file)
	} else {
		defFile, err := unembed(file)
		if err != nil {
			logger.Error("failed to unembed definition", zap.Error(err))
			return nil, err
		}

		defURI = toURI(defFile)
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

func unembed(file string) (string, error) {
	// assume stdlib
	var lib fs.File
	var defFile string
	var err error
	if strings.HasSuffix(file, ".bass") {
		lib, err = std.FS.Open(file)
		if err != nil {
			return "", fmt.Errorf("not found in /std/ fs: %w", err)
		}

		// stdlib is flat
		defFile = filepath.Join(defFile, "std", filepath.Base(file))
	} else if strings.Contains(file, "/bass/pkg/") {
		segs := strings.SplitN(file, "/bass/pkg/", 2)
		if len(segs) != 2 {
			return "", fmt.Errorf("impossible: quantum file path: %q", file)
		}

		pkgFile := segs[1]
		lib, err = pkg.FS.Open(pkgFile)
		if err != nil {
			return "", fmt.Errorf("not found in /pkg/ fs: %w", err)
		}

		defFile = filepath.Join(defFile, "pkg", pkgFile)
	}

	defer lib.Close()

	defPath, err := xdg.StateFile(filepath.Join("bass", "lsp", "defs", defFile))
	if err != nil {
		defPath = filepath.Join(os.TempDir(), "bass-lsp-defs", defFile)

		err = os.MkdirAll(filepath.Dir(defPath), 0755)
		if err != nil {
			return "", fmt.Errorf("could not create target path: %w", err)
		}
	}

	tmp, err := os.Create(defPath)
	if err != nil {
		return "", fmt.Errorf("create def file: %w", err)
	}

	defer tmp.Close()

	_, err = io.Copy(tmp, lib)
	if err != nil {
		return "", fmt.Errorf("write def file: %w", err)
	}

	return defPath, nil
}
