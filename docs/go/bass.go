package plugin

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image/color"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	svg "github.com/ajstarks/svgo"
	"github.com/alecthomas/chroma/styles"
	"github.com/vito/bass/demos"
	"github.com/vito/bass/pkg"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/bass/std"
	"github.com/vito/booklit"
	"github.com/vito/invaders"
	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
	"github.com/vito/vt100"
	"github.com/zeebo/xxh3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// used for stripping absolute paths when linking to code on GitHub
var projectRoot string

var docsLock = bass.NewHostPath(".", bass.ParseFileOrDirPath("./bass.lock"))

func init() {
	_, file, _, ok := runtime.Caller(0)
	if ok {
		projectRoot = filepath.Dir( // /
			filepath.Dir( // docs/
				filepath.Dir( // docs/go/
					file,
				),
			),
		) + string(filepath.Separator)
	}
}

func (plugin *Plugin) BassLiterate(alternating ...booklit.Content) (booklit.Content, error) {
	scope, stdoutSink, err := newScope()
	if err != nil {
		return nil, err
	}

	ctx, err := initBassCtx()
	if err != nil {
		return nil, err
	}

	var literate booklit.Sequence
	for i := 0; i < len(alternating); i++ {
		val := alternating[i]

		_, isCode := val.(booklit.Preformatted)
		if !isCode {
			plugin.paraID++

			tagName := fmt.Sprintf("s%sp%d", plugin.Section.Number(), plugin.paraID)
			title := booklit.String(fmt.Sprintf("\u00a7 %s \u00B6 %d", plugin.Section.Number(), plugin.paraID))

			literate = append(literate, plugin.literateClause(val, booklit.Target{
				TagName:  tagName,
				Location: plugin.Section.InvokeLocation,
				Title:    title,
				Content:  val,
			}))

			continue
		}

		literate = append(literate, booklit.LazyBlock(func() (booklit.Content, error) {
			stdoutSink.Reset()

			res, vterm := withProgress(ctx, "eval", func(ctx context.Context) (bass.Value, error) {
				file := bass.NewInMemoryFile(fmt.Sprintf("literate-%d", i), val.String())
				return bass.EvalFSFile(ctx, scope, file)
			})

			code, err := plugin.codeAndOutput(val, res, stdoutSink, vterm)
			if err != nil {
				return nil, err
			}

			return booklit.Styled{
				Style:   "literate-code",
				Block:   true,
				Content: code,
			}, nil
		}))
	}

	return booklit.Styled{
		Style:   "literate",
		Block:   true,
		Content: literate,
	}, nil
}

func (plugin *Plugin) Demos(demos ...booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "demos",
		Block:   true,
		Content: booklit.Sequence(demos),
	}
}

func (plugin *Plugin) DemoLiterate(name booklit.Content, literate ...booklit.Content) (booklit.Content, error) {
	content, err := plugin.BassLiterate(literate...)
	if err != nil {
		return nil, err
	}

	return booklit.Styled{
		Style:   "demo-literate",
		Block:   true,
		Content: content,
		Partials: booklit.Partials{
			"ID":   plugin.toggleId(name.String()),
			"Name": name,
		},
	}, nil
}

func (plugin *Plugin) Demo(demoFn string) booklit.Content {
	return booklit.LazyBlock(func() (booklit.Content, error) {
		demo, err := demos.FS.Open(demoFn)
		if err != nil {
			return nil, err
		}

		source, err := io.ReadAll(demo)
		if err != nil {
			return nil, err
		}

		source = bytes.TrimRight(source, "\n")
		source = bytes.TrimPrefix(source, []byte("#!/usr/bin/env bass\n"))

		stdoutSink := bass.NewInMemorySink()
		scope := bass.NewRunScope(bass.Ground, bass.RunState{
			Dir:    bass.NewFSDir(demos.FS),
			Stdout: bass.NewSink(stdoutSink),
			Stdin:  bass.NewSource(bass.NewInMemorySource()),
		})

		scope.Set("*memos*", docsLock)

		ctx, err := initBassCtx()
		if err != nil {
			return nil, err
		}

		demoPath := path.Join("demos", demoFn)

		_, vterm := withProgress(ctx, demoPath, func(ctx context.Context) (bass.Value, error) {
			file := bass.NewInMemoryFile(demoFn, string(source))
			res, err := bass.EvalFSFile(ctx, scope, file)
			if err != nil {
				return nil, err
			}

			err = bass.RunMain(ctx, scope)
			if err != nil {
				return nil, err
			}

			return res, nil
		})

		code, err := plugin.codeAndOutput(
			booklit.Preformatted{booklit.String(source)},
			nil, // don't show result
			stdoutSink,
			vterm,
		)
		if err != nil {
			return nil, err
		}

		return booklit.Styled{
			Style:   "bass-demo",
			Content: code,
			Partials: booklit.Partials{
				"Path": booklit.String(demoPath),
			},
		}, nil
	})
}

func (plugin *Plugin) GroundDocs() (booklit.Content, error) {
	return plugin.scopeDocs("", bass.Ground)
}

func (plugin *Plugin) ScriptDocs() (booklit.Content, error) {
	scp := bass.NewRunScope(bass.NewEmptyScope(), bass.RunState{
		Dir: bass.NewHostDir("."),
		Env: bass.Bindings{"SECRET_TOKEN": bass.String("im a spooky value")}.Scope(),
	})
	return plugin.scopeDocs("script", scp.Parents[0])
}

func (plugin *Plugin) StdlibDocs(name string, source booklit.Content) (booklit.Content, error) {
	res, cao, err := plugin.bassEval(source)
	if err != nil {
		return nil, err
	}

	var mod *bass.Scope
	if err := res.Decode(&mod); err != nil {
		return nil, err
	}

	docs, err := plugin.scopeDocs(name, mod)
	if err != nil {
		return nil, err
	}

	return booklit.Sequence{cao, docs}, nil
}

func (plugin *Plugin) codeAndOutput(
	code booklit.Content,
	res bass.Value,
	stdoutSink *bass.InMemorySink,
	vterm *vt100.VT100,
) (booklit.Content, error) {
	syntax, err := plugin.BassAutolink(code)
	if err != nil {
		return nil, err
	}

	var result booklit.Content
	if res != nil { // (might be nil for err case)
		result, err = plugin.renderValue(res)
		if err != nil {
			return nil, err
		}
	}

	stdoutBuf := new(bytes.Buffer)
	enc := bass.NewEncoder(stdoutBuf)
	enc.SetIndent("", "  ")
	for _, val := range stdoutSink.Values {
		enc.Encode(val)
	}

	var stdout booklit.Content
	if stdoutBuf.Len() > 0 {
		stdout, err = plugin.SyntaxTransform(
			"json",
			booklit.Styled{
				Style:   booklit.StyleVerbatim,
				Block:   true,
				Content: booklit.String(stdoutBuf.String()),
			},
			styles.Fallback,
		)
		if err != nil {
			return nil, err
		}
	}

	return booklit.Styled{
		Style:   "code-and-output",
		Content: syntax,
		Block:   true,
		Partials: booklit.Partials{
			"ID":          plugin.toggleId(code.String()),
			"Result":      result,
			"Stderr":      ansiTerm(vterm),
			"StderrLines": booklit.String(strconv.Itoa(vterm.UsedHeight())),
			"Stdout":      stdout,
		},
	}, nil
}

func (plugin *Plugin) toggleId(str string) booklit.Content {
	plugin.toggleCount++
	return booklit.String(fmt.Sprintf("%x-%d", xxh3.HashString(str), plugin.toggleCount))
}

func (plugin *Plugin) bassEval(source booklit.Content) (bass.Value, booklit.Content, error) {
	scope, stdoutSink, err := newScope()
	if err != nil {
		return nil, nil, err
	}

	ctx, err := initBassCtx()
	if err != nil {
		return nil, nil, err
	}

	vterm := newTerm()
	evalCtx := ioctx.StderrToContext(ctx, vterm)
	evalCtx = zapctx.ToContext(evalCtx, initZap(vterm))
	file := bass.NewInMemoryFile("docs-eval", source.String())
	res, err := bass.EvalFSFile(evalCtx, scope, file)
	if err != nil {
		return nil, nil, err
	}

	content, err := plugin.codeAndOutput(source, res, stdoutSink, vterm)
	if err != nil {
		return nil, nil, err
	}

	return res, content, nil
}

var runtimePool bass.RuntimePool
var runtimeOnce sync.Once

func initBassCtx() (context.Context, error) {
	ctx := context.Background()

	var err error
	runtimeOnce.Do(func() {
		runtimePool, err = runtimes.NewPool(ctx, &bass.Config{
			Runtimes: []bass.RuntimeConfig{
				{
					Runtime:  runtimes.BuildkitName,
					Platform: bass.LinuxPlatform,
					Config: bass.Bindings{
						"disable_cache": bass.Bool(os.Getenv("DISABLE_CACHE") != ""),
					}.Scope(),
				},
			},
		})
	})
	if err != nil {
		return nil, err
	}

	ctx = bass.WithRuntimePool(ctx, runtimePool)
	ctx = bass.WithTrace(ctx, &bass.Trace{})

	return ctx, nil
}

func newScope() (*bass.Scope, *bass.InMemorySink, error) {
	tmp, err := os.MkdirTemp("", "bass-scope")
	if err != nil {
		return nil, nil, err
	}

	stdoutSink := bass.NewInMemorySink()
	scope := bass.NewRunScope(bass.Ground, bass.RunState{
		Dir:    bass.NewHostDir(tmp),
		Stdout: bass.NewSink(stdoutSink),
		Stdin:  bass.NewSource(bass.NewInMemorySource()),
	})

	scope.Set("*memos*", docsLock)

	return scope, stdoutSink, nil
}

func (plugin *Plugin) literateClause(content booklit.Content, target booklit.Target) booklit.Content {
	return booklit.Styled{
		Style:   "literate-clause",
		Content: content,
		Partials: booklit.Partials{
			"Target": booklit.Styled{
				Style:   "clause-target",
				Content: target,
				Partials: booklit.Partials{
					"Reference": &booklit.Reference{
						Section: plugin.Section,
						TagName: target.TagName,
					},
				},
			},
		},
	}
}

func (plugin *Plugin) bindingTag(ns string, sym bass.Symbol) string {
	if ns == "" {
		return "binding-" + string(sym)
	} else {
		return "binding-" + ns + "-" + string(sym)
	}
}

func (plugin *Plugin) scopeDocs(ns string, scope *bass.Scope) (booklit.Content, error) {
	var content booklit.Sequence

	type indexedSymbol struct {
		Binding    bass.Symbol
		Desc       string
		Deprecated string
	}

	indexed := map[string][]indexedSymbol{}

	// NB: this doesn't recurse, otherwise we'd document Ground a million times
	for _, sym := range scope.Order {
		val, found := scope.Get(sym)
		if !found {
			// this should never happen
			continue
		}

		binding, err := plugin.bindingDocs(ns, scope, sym, val)
		if err != nil {
			return nil, err
		}

		content = append(content, binding)

		var index string
		fst := rune(sym[0])
		if unicode.IsLetter(fst) {
			index = string(fst)
		} else {
			index = "*/-"
		}

		var doc string
		var ann bass.Annotated
		if err := val.Decode(&ann); err == nil {
			if err := ann.Meta.GetDecode(bass.DocMetaBinding, &doc); err == nil {
				doc = strings.Split(doc, "\n\n")[0]
			} else {
				doc = bass.Details(ann.Value)
			}
		}

		var deprecated string
		if val, has := ann.Meta.Get(bass.DeprecatedMetaBinding); has {
			if err := val.Decode(&deprecated); err != nil {
				return nil, fmt.Errorf("deprecation: %w", err)
			}
		}

		indexed[index] = append(indexed[index], indexedSymbol{
			Binding:    sym,
			Desc:       doc,
			Deprecated: deprecated,
		})
	}

	type index struct {
		Key      string
		Bindings []indexedSymbol
	}

	var indexes []index
	for key, syms := range indexed {
		sort.Slice(syms, func(i, j int) bool {
			return syms[i].Binding < syms[j].Binding
		})

		indexes = append(indexes, index{key, syms})
	}
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].Key < indexes[j].Key
	})

	var rendered booklit.Sequence
	for _, idx := range indexes {
		bindings := make(booklit.Sequence, len(idx.Bindings))
		for i, sym := range idx.Bindings {
			bindings[i] = booklit.Styled{
				Style: "module-index-binding",
				Block: true,
				Content: &booklit.Reference{
					Section: plugin.Section,
					TagName: plugin.bindingTag(ns, sym.Binding),
				},
				Partials: booklit.Partials{
					"Description": booklit.String(sym.Desc),
					"Deprecated":  booklit.String(sym.Deprecated),
				},
			}
		}

		rendered = append(rendered, booklit.Styled{
			Style:   "module-index",
			Block:   true,
			Content: bindings,
			Partials: booklit.Partials{
				"Key": booklit.String(idx.Key),
			},
		})
	}

	return booklit.Styled{
		Style:   "module",
		Block:   true,
		Content: content,
		Partials: booklit.Partials{
			"Index": rendered,
		},
	}, nil
}

func (plugin *Plugin) bindingDocs(ns string, scope *bass.Scope, sym bass.Symbol, val bass.Value) (booklit.Content, error) {
	var body booklit.Content = booklit.Empty
	var deprecation booklit.Content = booklit.Empty
	var loc bass.Range
	var ann bass.Annotated
	if err := val.Decode(&ann); err == nil {
		_ = loc.FromMeta(ann.Meta)

		var deprecated string
		if err := ann.Meta.GetDecode(bass.DeprecatedMetaBinding, &deprecated); err == nil {
			deprecation, err = plugin.renderDocs(deprecated)
			if err != nil {
				return nil, err
			}
		}

		var docs string
		if err := ann.Meta.GetDecode(bass.DocMetaBinding, &docs); err == nil {
			body, err = plugin.renderDocs(docs)
			if err != nil {
				return nil, err
			}
		}
	}

	var predicates booklit.Sequence
	for _, pred := range bass.Predicates(val) {
		predicates = append(predicates, booklit.String(pred))
	}

	var inner bass.Value
	var app bass.Applicative
	if err := val.Decode(&app); err == nil {
		inner = app.Unwrap()
	} else {
		inner = val
	}

	var signature, value, startLine, endLine booklit.Content

	var op *bass.Operative
	var builtin *bass.Builtin
	if err := inner.Decode(&op); err == nil {
		form := bass.Pair{
			A: sym,
			D: op.Bindings,
		}

		signature, err = plugin.Bass(booklit.String(form.String()))
		if err != nil {
			return nil, err
		}
	} else if err := inner.Decode(&builtin); err == nil {
		form := bass.Pair{
			A: sym,
			D: builtin.Formals,
		}

		signature, err = plugin.Bass(booklit.String(form.String()))
		if err != nil {
			return nil, err
		}
	} else {
		signature, err = plugin.Bass(booklit.String(sym.String()))
		if err != nil {
			return nil, err
		}

		value, err = plugin.renderValue(val)
		if err != nil {
			return nil, err
		}
	}

	start := loc.Start.Ln
	startLine = booklit.String(strconv.Itoa(start))

	end := loc.End.Ln
	if end > start && end-start < 10 {
		// don't highlight too much; just highlight if it's short
		// enough to be hard to locate
		endLine = booklit.String(strconv.Itoa(end))
	}

	if loc.File == nil {
		return nil, fmt.Errorf("binding has no file location: %s", sym)
	}

	var path string
	var fsp *bass.FSPath
	if err := loc.File.Decode(&fsp); err == nil {
		switch fsp.FS {
		case std.FS:
			path = "std/" + fsp.Path.Slash()
		case pkg.FS:
			path = "pkg/" + fsp.Path.Slash()
		default:
			return nil, fmt.Errorf("unknown fs for binding '%s'", sym)
		}
	} else {
		return nil, fmt.Errorf("get binding path: %w", err)
	}

	tagName := plugin.bindingTag(ns, sym)

	hl, err := plugin.Bass(booklit.String(sym))
	if err != nil {
		return nil, err
	}

	return booklit.Styled{
		Style:   "bass-binding",
		Block:   true,
		Content: signature,
		Partials: booklit.Partials{
			"Body":       body,
			"Value":      value,
			"Predicates": predicates,
			"Path":       booklit.String(path),
			"StartLine":  startLine,
			"EndLine":    endLine,
			"Reference": &booklit.Reference{
				Section: plugin.Section,
				TagName: tagName,
			},
			"Target": booklit.Target{
				TagName:  tagName,
				Location: plugin.Section.InvokeLocation,
				Title:    hl,
				Content:  body,
			},
			"Deprecation": deprecation,
		},
	}, nil
}

var bindingRe = regexp.MustCompile(`\[[a-zA-Z!$&*_+=|<.>?\-;]+?\]`)

func (plugin *Plugin) renderDocs(docs string) (booklit.Content, error) {
	lines := strings.Split(docs, "\n")

	var body booklit.Sequence

	for _, line := range lines {
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "=> ") {
			example := strings.TrimPrefix(line, "=> ")

			body = append(body, booklit.Preformatted{
				booklit.String(example),
			})
		} else {
			seq := booklit.Sequence{}

			matches := bindingRe.FindAllStringIndex(line, -1)
			if matches == nil {
				seq = append(seq, booklit.String(line))
			} else {
				last := 0

				for _, match := range matches {
					start, end := match[0], match[1]

					if last < start {
						prefix := line[last:start]
						seq = append(seq, booklit.String(prefix))
					}

					binding := line[start+1 : end-1]
					seq = append(seq, plugin.B(booklit.String(binding)))

					last = end
				}

				if last < len(line) {
					seq = append(seq, booklit.String(line[last:]))
				}
			}

			body = append(body, booklit.Paragraph{
				seq,
			})
		}
	}

	return plugin.BassLiterate(body...)
}

func formatPartials(f vt100.Format) booklit.Partials {
	return booklit.Partials{
		"Foreground": booklit.String(colorName(f.Fg, f.FgBright)),
		"Background": booklit.String(colorName(f.Bg, f.BgBright)),
		"Intensity":  booklit.String(intensityName(f.Intensity)),
	}
}

func colorName(f color.RGBA, bright bool) string {
	var color string
	switch f {
	case vt100.Black:
		color = "black"
	case vt100.Red:
		color = "red"
	case vt100.Green:
		color = "green"
	case vt100.Yellow:
		color = "yellow"
	case vt100.Blue:
		color = "blue"
	case vt100.Magenta:
		color = "magenta"
	case vt100.Cyan:
		color = "cyan"
	case vt100.White:
		color = "white"
	}

	if bright {
		color = "bright-" + color
	}

	return color
}

func intensityName(intensity vt100.Intensity) string {
	switch intensity {
	case vt100.Bold:
		return "bold"
	case vt100.Dim:
		return "dim"
	default:
		return ""
	}
}

func ansiTerm(vterm *vt100.VT100) booklit.Content {
	var output booklit.Sequence

	used := vterm.UsedHeight()

	var chars int
	for y, row := range vterm.Content {
		if y >= used {
			break
		}

		var lineSeq booklit.Sequence

		chunk := new(bytes.Buffer)

		var chunkFormat vt100.Format
		for x, c := range row {
			f := vterm.Format[y][x]

			if f != chunkFormat {
				if chunk.Len() > 0 {
					chars += chunk.Len()

					lineSeq = append(lineSeq, booklit.Styled{
						Style:    "ansi",
						Content:  booklit.String(chunk.String()),
						Partials: formatPartials(chunkFormat),
					})
				}

				chunkFormat = f
				chunk.Reset()
			}

			_, err := chunk.WriteRune(c)
			if err != nil {
				panic(err)
			}
		}

		if chunk.Len() > 0 {
			content := strings.TrimRight(chunk.String(), " ")
			if content != "" {
				chars += len(content)

				lineSeq = append(lineSeq, booklit.Styled{
					Style:    "ansi",
					Content:  booklit.String(content),
					Partials: formatPartials(chunkFormat),
				})
			}
		}

		output = append(output, booklit.Styled{
			Style:   "ansi-line",
			Block:   true,
			Content: lineSeq,
		})
	}

	if chars == 0 {
		return booklit.Empty
	}

	return booklit.Styled{
		Style:   booklit.StyleVerbatim,
		Block:   true,
		Content: output,
	}
}

func initZap(dest io.Writer) *zap.Logger {
	zapcfg := zap.NewDevelopmentEncoderConfig()
	zapcfg.EncodeLevel = zapcore.LowercaseColorLevelEncoder

	// don't need timestamps there
	zapcfg.EncodeTime = nil

	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zapcfg),
		zapcore.AddSync(dest),
		zapcore.DebugLevel,
	))
}

func (plugin *Plugin) renderValue(val bass.Value) (booklit.Content, error) {
	var thunkPath bass.ThunkPath
	if err := val.Decode(&thunkPath); err == nil {
		return plugin.renderThunkPath(thunkPath)
	}

	var addr bass.ThunkAddr
	if err := val.Decode(&addr); err == nil {
		return plugin.renderThunkAddr(addr)
	}

	var wl bass.Thunk
	if err := val.Decode(&wl); err == nil {
		return plugin.renderThunk(wl)
	}

	var obj *bass.Scope
	if err := val.Decode(&obj); err == nil {
		return plugin.renderScope(obj)
	}

	var list bass.List
	if err := val.Decode(&list); err == nil && bass.IsList(list) {
		return plugin.renderList(list)
	}

	return plugin.Bass(booklit.String(val.String()))
}

func (plugin *Plugin) renderScope(scope *bass.Scope) (booklit.Content, error) {
	if scope.Name != "" {
		return booklit.Styled{
			Style:   booklit.StyleVerbatim,
			Content: booklit.String(scope.String()),
		}, nil
	}

	var rows booklit.Sequence
	for _, k := range scope.Order {
		v := scope.Bindings[k]

		keyContent, err := plugin.Bass(booklit.String(k.Keyword().String()))
		if err != nil {
			return nil, err
		}

		subContent, err := plugin.renderValue(v)
		if err != nil {
			return nil, err
		}

		rows = append(rows, booklit.Styled{
			Style: "bass-scope-binding",
			Content: booklit.Sequence{
				keyContent,
				subContent,
			},
		})
	}

	var parents booklit.Sequence
	for _, p := range scope.Parents {
		parent, err := plugin.renderScope(p)
		if err != nil {
			return nil, err
		}

		parents = append(parents, parent)
	}

	if len(parents) > 0 {
		rows = append(rows, booklit.Styled{
			Style:   "bass-scope-parents",
			Content: parents,
		})
	}

	return booklit.Styled{
		Style:   "bass-scope",
		Content: rows,
	}, nil
}

func (plugin *Plugin) renderList(list bass.List) (booklit.Content, error) {
	var rows booklit.Sequence
	err := bass.Each(list, func(val bass.Value) error {
		subContent, err := plugin.renderValue(val)
		if err != nil {
			return err
		}

		rows = append(rows, booklit.Sequence{
			subContent,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return booklit.Styled{
		Style:   "bass-list",
		Content: rows,
	}, nil
}

func (plugin *Plugin) renderThunkPath(wlp bass.ThunkPath) (booklit.Content, error) {
	return plugin.renderThunk(wlp.Thunk, wlp.Path.ToValue())
}

func (plugin *Plugin) renderThunkAddr(addr bass.ThunkAddr) (booklit.Content, error) {
	return plugin.renderThunk(addr.Thunk, bass.Symbol(addr.Port))
}

func (plugin *Plugin) renderThunk(thunk bass.Thunk, pathOptional ...bass.Value) (booklit.Content, error) {
	invader, err := thunk.Avatar()
	if err != nil {
		return nil, err
	}

	avatarSvg := new(bytes.Buffer)
	canvas := svg.New(avatarSvg)

	cellSize := 6
	canvas.Startview(
		cellSize*invaders.Width,
		cellSize*invaders.Height,
		0,
		0,
		cellSize*invaders.Width,
		cellSize*invaders.Height,
	)
	canvas.Group()

	for row := range invader {
		y := row * cellSize

		for col := range invader[row] {
			x := col * cellSize
			shade := invader[row][col]

			var color string
			switch shade {
			case invaders.Background:
				color = "transparent"
			case invaders.Shade1:
				color = "var(--base08)"
			case invaders.Shade2:
				color = "var(--base09)"
			case invaders.Shade3:
				color = "var(--base0A)"
			case invaders.Shade4:
				color = "var(--base0B)"
			case invaders.Shade5:
				color = "var(--base0C)"
			case invaders.Shade6:
				color = "var(--base0D)"
			case invaders.Shade7:
				color = "var(--base0E)"
			default:
				return nil, fmt.Errorf("invalid shade: %v", shade)
			}

			canvas.Rect(
				x, y,
				cellSize, cellSize,
				fmt.Sprintf("fill: %s", color),
			)
		}
	}

	canvas.Gend()
	canvas.End()

	thunkScope := bass.NewEmptyScope()
	if thunk.Image != nil {
		thunkScope.Set("image", thunk.Image.ToValue())
	}
	if thunk.Insecure {
		thunkScope.Set("insecure", bass.Bool(thunk.Insecure))
	}
	if len(thunk.Args) > 0 {
		thunkScope.Set("args", bass.NewList(thunk.Args...))
	}
	if len(thunk.Stdin) > 0 {
		thunkScope.Set("stdin", bass.NewList(thunk.Stdin...))
	}
	if thunk.Env != nil {
		thunkScope.Set("env", thunk.Env)
	}
	if thunk.Dir != nil {
		thunkScope.Set("dir", thunk.Dir.ToValue())
	}
	if len(thunk.Mounts) > 0 {
		var mounts []bass.Value
		for _, m := range thunk.Mounts {
			mountScope := bass.NewEmptyScope()
			mountScope.Set("source", m.Source.ToValue())
			mountScope.Set("target", m.Target.ToValue())
			mounts = append(mounts, mountScope)
		}
		thunkScope.Set("mounts", bass.NewList(mounts...))
	}
	if thunk.Labels != nil {
		thunkScope.Set("labels", thunk.Labels)
	}

	scope, err := plugin.renderScope(thunkScope)
	if err != nil {
		return nil, err
	}

	var run booklit.Content
	if len(thunk.Args) > 0 {
		run, err = plugin.renderValue(thunk.Args[0])
	} else {
		run = booklit.String("<no command>")
	}
	if err != nil {
		return nil, err
	}

	hash, err := thunk.Hash()
	if err != nil {
		return nil, err
	}

	plugin.toggleCount++

	var path booklit.Content
	if len(pathOptional) > 0 {
		path, err = plugin.renderValue(pathOptional[0])
		if err != nil {
			return nil, err
		}
	}

	return booklit.Styled{
		Style:   "bass-thunk",
		Content: booklit.String(avatarSvg.String()),
		Partials: booklit.Partials{
			"ID":    booklit.String(fmt.Sprintf("%s-%d", hash, plugin.toggleCount)),
			"Run":   run,
			"Path":  path,
			"Scope": scope,
		},
	}, nil
}

type kv struct {
	k bass.Symbol
	v bass.Value
}

// NB: stderr panes fit 90 characters at the moment on the frontpage, so just
// keep this aligned
const maxTermWidth = 90
const maxTermHeight = 1000

func newTerm() *vt100.VT100 {
	return vt100.NewVT100(maxTermHeight, maxTermWidth)
}

func withProgress(ctx context.Context, name string, f func(context.Context) (bass.Value, error)) (bass.Value, *vt100.VT100) {
	statuses, progW := progrock.Pipe()

	recorder := progrock.NewRecorder(progW)
	ctx = progrock.RecorderToContext(ctx, recorder)

	vterm := newTerm()
	model := ui.NewModel(func() {}, vterm, cli.ProgressUI, true)

	// NB: having this exactly match the stderr width seems to cause vterm line
	// doubling, so subtract 1. maybe this has to do with the scrollbar?
	model.SetWindowSize(maxTermWidth-1, maxTermHeight)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			status, ok := statuses.ReadStatus()
			if !ok {
				return
			}

			model.StatusUpdate(status)
		}
	}()

	// wire up logs to vertex
	ctx = zapctx.ToContext(ctx, initZap(vterm))

	// wire up stderr for (log), (debug), etc.
	ctx = ioctx.StderrToContext(ctx, vterm)

	res, err := f(ctx)

	progW.Close()
	wg.Wait()

	model.Print(vterm)

	if err != nil {
		cli.WriteError(ctx, err)
	}

	return res, vterm
}
