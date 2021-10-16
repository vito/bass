package plugin

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image/color"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	svg "github.com/ajstarks/svgo"
	"github.com/opencontainers/go-digest"
	"github.com/vito/bass"
	"github.com/vito/bass/demos"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/runtimes"
	"github.com/vito/bass/std"
	"github.com/vito/bass/zapctx"
	"github.com/vito/booklit"
	"github.com/vito/invaders"
	"github.com/vito/progrock"
	"github.com/vito/vt100"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// used for stripping absolute paths when linking to code on GitHub
var projectRoot string

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

func (plugin *Plugin) codeAndOutput(
	code booklit.Content,
	res bass.Value,
	stdoutSink *bass.InMemorySink,
	vterm *vt100.VT100,
) (booklit.Content, error) {
	syntax, err := plugin.Bass(code)
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

	var stdout booklit.Sequence
	for _, val := range stdoutSink.Values {
		rendered, err := plugin.renderValue(val)
		if err != nil {
			return nil, err
		}

		stdout = append(stdout, rendered)
	}

	return booklit.Styled{
		Style:   "code-and-output",
		Content: syntax,
		Block:   true,
		Partials: booklit.Partials{
			"Result": result,
			"Stderr": ansiTerm(vterm),
			"Stdout": stdout,
		},
	}, nil
}

func (plugin *Plugin) BassEval(source booklit.Content) (booklit.Content, error) {
	scope, stdoutSink, err := newScope()
	if err != nil {
		return nil, err
	}

	ctx, err := initBassCtx()
	if err != nil {
		return nil, err
	}

	res, vterm := withProgress(ctx, "eval", func(ctx context.Context, rec *progrock.VertexRecorder) (bass.Value, error) {
		return bass.EvalString(ctx, scope, source.String(), "(docs)")
	})

	return plugin.codeAndOutput(source, res, stdoutSink, vterm)
}

func initBassCtx() (context.Context, error) {
	ctx := context.Background()

	pool, err := runtimes.NewPool(&bass.Config{
		Runtimes: []bass.RuntimeConfig{
			{
				Runtime:  runtimes.DockerName,
				Platform: bass.LinuxPlatform,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	ctx = bass.WithRuntime(ctx, pool)
	ctx = bass.WithTrace(ctx, &bass.Trace{})

	return ctx, nil
}

func newScope() (*bass.Scope, *bass.InMemorySink, error) {
	stdoutSink := bass.NewInMemorySink()
	scope := runtimes.NewScope(bass.Ground, runtimes.RunState{
		Dir:    bass.HostPath{Path: "."},
		Args:   bass.NewList(),
		Stdout: bass.NewSink(stdoutSink),
		Stdin:  bass.NewSource(bass.NewInMemorySource()),
	})

	return scope, stdoutSink, nil
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

			literate = append(literate, booklit.Styled{
				Style:   "literate-clause",
				Content: val,
				Partials: booklit.Partials{
					"Target": booklit.Styled{
						Style: "clause-target",
						Content: booklit.Target{
							TagName:  tagName,
							Location: plugin.Section.InvokeLocation,
							Title:    title,
							Content:  val,
						},
						Partials: booklit.Partials{
							"Reference": &booklit.Reference{
								TagName: tagName,
							},
						},
					},
				},
			})

			continue
		}

		res, vterm := withProgress(ctx, "literate", func(ctx context.Context, rec *progrock.VertexRecorder) (bass.Value, error) {
			return bass.EvalString(ctx, scope, val.String(), "(docs)")
		})

		code, err := plugin.codeAndOutput(val, res, stdoutSink, vterm)
		if err != nil {
			return nil, err
		}

		literate = append(literate, booklit.Styled{
			Style:   "literate-code",
			Block:   true,
			Content: code,
		})
	}

	return booklit.Styled{
		Style:   "literate",
		Block:   true,
		Content: literate,
	}, nil
}

func (plugin *Plugin) Demo(demoFn string) (booklit.Content, error) {
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

	scope, stdoutSink, err := newScope()
	if err != nil {
		return nil, err
	}

	ctx, err := initBassCtx()
	if err != nil {
		return nil, err
	}

	demoPath := path.Join("demos", demoFn)

	_, vterm := withProgress(ctx, demoPath, func(ctx context.Context, rec *progrock.VertexRecorder) (bass.Value, error) {
		return bass.EvalString(ctx, scope, string(source), demoFn)
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
}

func (plugin *Plugin) bindingDocs(scope *bass.Scope, sym bass.Symbol, body booklit.Content, loc bass.Range) (booklit.Content, error) {
	val, found := scope.Get(sym)
	if !found {
		return booklit.Empty, nil
	}

	var predicates booklit.Sequence
	for _, pred := range bass.Predicates(val) {
		predicates = append(predicates, booklit.String(pred))
	}

	var app bass.Applicative
	if err := val.Decode(&app); err == nil {
		val = app.Unwrap()
	}

	var signature, value, startLine, endLine booklit.Content

	var op *bass.Operative
	var builtin *bass.Builtin
	if err := val.Decode(&op); err == nil {
		form := bass.Pair{
			A: sym,
			D: op.Bindings,
		}

		signature, err = plugin.Bass(booklit.String(form.String()))
		if err != nil {
			return nil, err
		}
	} else if err := val.Decode(&builtin); err == nil {
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

	path := loc.Start.File
	if filepath.IsAbs(path) {
		path = filepath.ToSlash(strings.TrimPrefix(path, projectRoot))
	} else {
		path = "std/" + path
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
			"Target": booklit.Target{
				TagName:  string(sym),
				Location: plugin.Section.InvokeLocation,
				Title: booklit.Styled{
					Style:   booklit.StyleVerbatim,
					Content: booklit.String(sym),
				},
				Content: body,
			},
		},
	}, nil
}

func (plugin *Plugin) scopeDocs(scope *bass.Scope) (booklit.Content, error) {
	var content booklit.Sequence
	for _, annotated := range scope.Commentary {
		lines := strings.Split(annotated.Comment, "\n")

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
				body = append(body, booklit.Paragraph{
					booklit.String(line),
				})
			}
		}

		literate, err := plugin.BassLiterate(body...)
		if err != nil {
			return nil, err
		}

		var sym bass.Symbol
		if err := annotated.Value.Decode(&sym); err == nil {
			binding, err := plugin.bindingDocs(scope, sym, literate, annotated.Range)
			if err != nil {
				return nil, err
			}
			content = append(content, binding)
		} else {
			content = append(content, literate)
		}
	}

	return booklit.Styled{
		Style:   "bass-commentary",
		Block:   true,
		Content: content,
	}, nil
}

func (plugin *Plugin) GroundDocs() (booklit.Content, error) {
	return plugin.scopeDocs(bass.Ground)
}

func (plugin *Plugin) StdlibDocs(path string) (booklit.Content, error) {
	scope := bass.NewStandardScope()

	ctx, err := initBassCtx()
	if err != nil {
		return nil, err
	}

	_, err = bass.EvalFSFile(ctx, scope, std.FS, path)
	if err != nil {
		return nil, err
	}

	return plugin.scopeDocs(scope)
}

func formatPartials(f vt100.Format) booklit.Partials {
	ps := booklit.Partials{}

	fg := colorName(f.Fg)
	if f.Intensity == vt100.Bright {
		fg = "bright-" + fg
	}

	ps["Foreground"] = booklit.String(fg)

	// NB: it's apparently impossible to represent a bright background color?
	ps["Background"] = booklit.String(colorName(f.Bg))

	return ps
}

func colorName(f color.RGBA) string {
	switch f {
	case vt100.Black:
		return "black"
	case vt100.Red:
		return "red"
	case vt100.Green:
		return "green"
	case vt100.Yellow:
		return "yellow"
	case vt100.Blue:
		return "blue"
	case vt100.Magenta:
		return "magenta"
	case vt100.Cyan:
		return "cyan"
	case vt100.White:
		return "white"
	default:
		return "unknown"
	}
}

func ansiTerm(vterm *vt100.VT100) booklit.Content {
	var output booklit.Sequence

	used := vterm.UsedHeight()

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
				if chunk.Len() > 0 && strings.TrimSpace(chunk.String()) != "" {
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

	if len(output) == 0 {
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
	var wlp bass.WorkloadPath
	if err := val.Decode(&wlp); err == nil {
		return plugin.renderWorkloadPath(wlp)
	}

	// handle constructed workloads
	var wl bass.Workload
	if err := val.Decode(&wl); err == nil {
		return plugin.renderWorkload(wl)
	}

	var obj *bass.Scope
	if err := val.Decode(&obj); err == nil {
		return plugin.renderScope(obj)
	}

	var list bass.List
	if err := val.Decode(&list); err == nil && bass.IsList(list) {
		return plugin.renderList(list)
	}

	return plugin.Bass(booklit.String(fmt.Sprintf("%s", val)))
}

func (plugin *Plugin) renderScope(scope *bass.Scope) (booklit.Content, error) {
	if scope.Name != "" {
		return booklit.Styled{
			Style:   booklit.StyleVerbatim,
			Content: booklit.String(scope.String()),
		}, nil
	}

	// handle embedded workload paths
	var wlp bass.WorkloadPath
	if err := scope.Decode(&wlp); err == nil {
		return plugin.renderWorkloadPath(wlp)
	}

	var pairs pairs
	for k, v := range scope.Bindings {
		pairs = append(pairs, kv{k, v})
	}

	sort.Sort(pairs)

	var rows booklit.Sequence
	for _, kv := range pairs {
		k, v := kv.k, kv.v

		keyContent, err := plugin.Bass(booklit.String(k.String()))
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

func (plugin *Plugin) renderWorkloadPath(wlp bass.WorkloadPath) (booklit.Content, error) {
	return plugin.renderWorkload(wlp.Workload, wlp.Path.ToValue())
}

func (plugin *Plugin) renderWorkload(workload bass.Workload, pathOptional ...bass.Value) (booklit.Content, error) {
	invader, err := workload.Avatar()
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

	payload, err := bass.MarshalJSON(workload)
	if err != nil {
		return nil, err
	}

	var obj *bass.Scope
	err = bass.UnmarshalJSON(payload, &obj)
	if err != nil {
		return nil, err
	}

	scope, err := plugin.renderScope(obj)
	if err != nil {
		return nil, err
	}

	run, err := plugin.renderValue(workload.Path.ToValue())
	if err != nil {
		return nil, err
	}

	id, err := workload.SHA1()
	if err != nil {
		return nil, err
	}

	plugin.toggleID++

	var path booklit.Content
	if len(pathOptional) > 0 {
		path, err = plugin.renderValue(pathOptional[0])
		if err != nil {
			return nil, err
		}
	}

	return booklit.Styled{
		Style:   "bass-workload",
		Content: booklit.String(avatarSvg.String()),
		Partials: booklit.Partials{
			"ID":    booklit.String(fmt.Sprintf("%s-%d", id, plugin.toggleID)),
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

type pairs []kv

func (kvs pairs) Len() int { return len(kvs) }

func (kvs pairs) Less(i, j int) bool {
	return kvs[i].k < kvs[j].k
}

func (kvs pairs) Swap(i, j int) {
	kvs[i], kvs[j] = kvs[j], kvs[i]
}

func newTerm() *vt100.VT100 {
	return vt100.NewVT100(1000, 1000)
}

func withProgress(ctx context.Context, name string, f func(context.Context, *progrock.VertexRecorder) (bass.Value, error)) (bass.Value, *vt100.VT100) {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	statuses, progW := progrock.Pipe()

	recorder := progrock.NewRecorder(progW)
	defer recorder.Stop()

	ctx = progrock.RecorderToContext(ctx, recorder)

	vterm := newTerm()
	recorder.Display(ctx, "Playing", nil, vterm, statuses)

	var res bass.Value
	var err error

	bassVertex := recorder.Vertex(digest.Digest(name), fmt.Sprintf("[bass] %s", name))
	defer func() { bassVertex.Done(err) }()

	stderr := bassVertex.Stderr()

	// wire up logs to vertex
	ctx = zapctx.ToContext(ctx, initZap(stderr))

	// wire up stderr for (log), (debug), etc.
	ctx = ioctx.StderrToContext(ctx, stderr)

	res, err = f(ctx, bassVertex)
	if err != nil {
		bass.WriteError(ctx, err)
	}

	return res, vterm
}
