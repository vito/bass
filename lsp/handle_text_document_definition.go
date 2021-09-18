package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
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

func (h *langHandler) findTag(fname string, tag string) ([]Location, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	locations := []Location{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "!") {
			continue
		}
		token := strings.SplitN(text, "\t", 4)
		if len(token) < 4 {
			continue
		}
		if token[0] == tag {
			token[2] = strings.TrimRight(token[2], `;"`)
			fullpath := filepath.Clean(filepath.Join(h.rootPath, token[1]))
			b, err := ioutil.ReadFile(fullpath)
			if err != nil {
				continue
			}
			lines := strings.Split(string(b), "\n")
			if strings.HasPrefix(token[2], "/") {
				pattern := token[2]
				hasPrefix := strings.HasPrefix(pattern, "/^")
				if hasPrefix {
					pattern = strings.TrimLeft(pattern, "/^")
				}
				hasSuffix := strings.HasSuffix(pattern, "$/")
				if hasSuffix {
					pattern = strings.TrimRight(pattern, "$/")
				}
				for i, line := range lines {
					match := false
					if hasPrefix && hasSuffix && line == pattern {
						match = true
					} else if hasPrefix && strings.HasPrefix(line, pattern) {
						match = true
					} else if hasSuffix && strings.HasSuffix(line, pattern) {
						match = true
					}
					if match {
						locations = append(locations, Location{
							URI: toURI(fullpath),
							Range: Range{
								Start: Position{Line: i, Character: 0},
								End:   Position{Line: i, Character: 0},
							},
						})
					}
				}
			} else {
				i, err := strconv.Atoi(token[2])
				if err != nil {
					continue
				}
				locations = append(locations, Location{
					URI: toURI(fullpath),
					Range: Range{
						Start: Position{Line: i - 1, Character: 0},
						End:   Position{Line: i - 1, Character: 0},
					},
				})
			}
		}
	}
	return locations, nil
}

func (h *langHandler) findTagsFile(fname string) string {
	base := filepath.Clean(filepath.Dir(fname))
	for {
		_, err := os.Stat(filepath.Join(base, "tags"))
		if err == nil {
			break
		}
		if base == h.rootPath {
			base = ""
			break
		}
		tmp := filepath.Dir(base)
		if tmp == "" || tmp == base {
			base = ""
			break
		}
		base = tmp
	}
	return base
}

func (h *langHandler) definition(ctx context.Context, uri DocumentURI, params *DocumentDefinitionParams) ([]Location, error) {
	logger := zapctx.FromContext(ctx)

	f, ok := h.files[uri]
	if !ok {
		return nil, fmt.Errorf("document not found: %v", uri)
	}

	lines := strings.Split(f.Text, "\n")
	if params.Position.Line < 0 || params.Position.Line > len(lines) {
		return nil, fmt.Errorf("invalid position: %v", params.Position)
	}
	chars := utf16.Encode([]rune(lines[params.Position.Line]))
	if params.Position.Character < 0 || params.Position.Character > len(chars) {
		return nil, fmt.Errorf("invalid position: %v", params.Position)
	}
	prevPos := 0
	currPos := -1
	prevCls := unicodeclass.Invalid
	for i, char := range chars {
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

	tag := string(utf16.Decode(chars[prevPos:currPos]))

	logger = logger.With(zap.String("tag", tag))

	scope, found := h.scopes[uri]
	if !found {
		logger.Warn("scope not initialized", zap.String("uri", string(uri)))
		return nil, nil
	}

	annotated, found := scope.GetWithDoc(bass.Symbol(tag))
	if !found {
		logger.Debug("binding not found")
		return nil, nil
	}

	if annotated.Range.Start.File == "" {
		logger.Debug("no docs :(")
		return nil, nil
	}

	logger.Debug("found doc", zap.Any("range", annotated.Range))

	var defURI DocumentURI

	file := annotated.Range.Start.File
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
					Line:      annotated.Range.Start.Ln - 1,
					Character: annotated.Range.Start.Col,
				},
				End: Position{
					Line:      annotated.Range.End.Ln - 1,
					Character: annotated.Range.End.Col,
				},
			},
		},
	}, nil
}
