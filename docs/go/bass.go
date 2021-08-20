package plugin

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"

	svg "github.com/ajstarks/svgo"
	"github.com/aoldershaw/ansi"
	"github.com/vito/bass"
	"github.com/vito/bass/demos"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/runtimes"
	"github.com/vito/bass/std"
	"github.com/vito/bass/zapctx"
	"github.com/vito/booklit"
	"github.com/vito/invaders"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type intWriter struct {
	*ansi.Writer
}

func (writer intWriter) Write(p []byte) (int, error) {
	i64, err := writer.Writer.Write(p)
	if err != nil {
		return 0, err
	}

	return int(i64), nil
}

//go:embed stdlib.bass
var stdlib string

func (plugin *Plugin) BassLiterate(alternating ...booklit.Content) (booklit.Content, error) {
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

	stdout := bass.NewInMemorySink()
	env := runtimes.NewEnv(pool, runtimes.RunState{
		Dir:    bass.HostPath{Path: "."},
		Args:   bass.NewList(),
		Stdout: bass.NewSink(stdout),
		Stdin:  bass.NewSource(bass.NewInMemorySource()),
	})

	ctx := context.Background()

	_, err = bass.EvalString(ctx, env, stdlib)
	if err != nil {
		return nil, fmt.Errorf("init stdlib: %w", err)
	}

	var lines ansi.Lines
	writer := ansi.NewWriter(&lines)
	iw := io.MultiWriter(os.Stderr, intWriter{writer})

	// assign after stdlib so its output isn't in every example
	ctx = zapctx.ToContext(ctx, initZap(iw))
	ctx = ioctx.StderrToContext(ctx, iw)
	ctx = bass.WithTrace(ctx, &bass.Trace{})
	lines = nil

	var literate booklit.Sequence
	for i := 0; i < len(alternating); i++ {
		val := alternating[i]

		_, isCode := val.(booklit.Preformatted)
		if !isCode {
			literate = append(literate, val)
			continue
		}

		syntax, err := plugin.Bass(val)
		if err != nil {
			return nil, err
		}

		_, err = bass.EvalString(ctx, env, val.String(), "(docs)")
		if err != nil {
			bass.WriteError(ctx, iw, err)
		}

		var outputs booklit.Sequence
		for _, val := range stdout.Values {
			rendered, err := plugin.renderValue(val)
			if err != nil {
				return nil, err
			}

			outputs = append(outputs, rendered)
		}

		literate = append(literate, booklit.Styled{
			Style: "literate-code",
			Block: true,
			Content: booklit.Styled{
				Style:   "code-and-output",
				Content: syntax,
				Block:   true,
				Partials: booklit.Partials{
					"Stderr": ansiLines(lines),
					"Stdout": outputs,
				},
			},
		})

		lines = nil
	}

	return booklit.Styled{
		Style:   "literate",
		Block:   true,
		Content: literate,
	}, nil
}

func (plugin *Plugin) Demo(path string, literate ...booklit.Content) (booklit.Content, error) {
	demo, err := demos.FS.Open(path)
	if err != nil {
		return nil, err
	}

	source, err := io.ReadAll(demo)
	if err != nil {
		return nil, err
	}

	source = bytes.TrimRight(source, "\n")
	source = bytes.TrimPrefix(source, []byte("#!/usr/bin/env bass\n"))

	demoSegment := booklit.Preformatted{
		booklit.String(source),
	}

	content, err := plugin.BassLiterate(append(literate, demoSegment)...)
	if err != nil {
		return nil, err
	}

	return booklit.Styled{
		Style:   "bass-demo",
		Content: content,
		Partials: booklit.Partials{
			"Path": booklit.String(path),
		},
	}, nil
}

func (plugin *Plugin) StdlibDocs(path string) (booklit.Content, error) {
	ctx := context.Background()

	env := bass.NewStandardEnv()

	_, err := bass.EvalFSFile(ctx, env, std.FS, path)
	if err != nil {
		return nil, err
	}

	var content booklit.Sequence
	for _, annotated := range env.Commentary {
		lines := strings.Split(annotated.Comment, "\n")

		var symbol, signature, title, startLine, endLine booklit.Content

		var sym bass.Symbol
		if err := annotated.Value.Decode(&sym); err == nil {
			val, found := env.Get(sym)
			if found {
				isFn := false

				var app bass.Applicative
				if err := val.Decode(&app); err == nil {
					isFn = true
					val = app.Unwrap()
				}

				var op *bass.Operative
				if err := val.Decode(&op); err == nil {
					defsym := bass.Symbol("defop")
					if isFn {
						defsym = bass.Symbol("defn")
					}

					def := []bass.Value{
						defsym,
						sym,
						op.Formals,
					}

					if !isFn {
						def = append(def, op.Eformal)
					}

					signature, err = plugin.Bass(booklit.String(bass.NewList(def...).String()))
					if err != nil {
						return nil, err
					}
				} else {
					def := []bass.Value{
						bass.Symbol("def"),
						sym,
						val,
					}

					signature, err = plugin.Bass(booklit.String(bass.NewList(def...).String()))
					if err != nil {
						return nil, err
					}
				}
			}

			symbol = booklit.String(sym)

			title = booklit.String(lines[0])
			lines = lines[1:]

			start := annotated.Range.Start.Ln
			startLine = booklit.String(strconv.Itoa(start))

			end := annotated.Range.End.Ln
			if end > start && end-start < 10 {
				// don't highlight too much; just highlight if it's short
				// enough to be hard to locate
				endLine = booklit.String(strconv.Itoa(end))
			}
		}

		var body booklit.Sequence
		for _, line := range lines {
			if len(line) == 0 {
				continue
			}

			body = append(body, booklit.Paragraph{
				booklit.String(line),
			})
		}

		content = append(content, booklit.Styled{
			Style:   "bass-commented",
			Content: body,
			Partials: booklit.Partials{
				"Symbol":    symbol,
				"Signature": signature,
				"Title":     title,
				"Path":      booklit.String(path),
				"StartLine": startLine,
				"EndLine":   endLine,
			},
		})
	}

	return booklit.Styled{
		Style:   "bass-commentary",
		Content: content,
		Partials: booklit.Partials{
			"Path": booklit.String(path),
		},
	}, nil
}

func ansiLines(lines ansi.Lines) booklit.Content {
	var output booklit.Sequence
	var sawOutput bool
	for _, line := range lines {
		if len(line) == 0 && !sawOutput {
			continue
		}

		sawOutput = true

		var lineSeq booklit.Sequence
		for _, chunk := range line {
			lineSeq = append(lineSeq, booklit.Styled{
				Style:   "ansi",
				Content: booklit.String(chunk.Data),
				Partials: booklit.Partials{
					"Foreground": booklit.String(chunk.Style.Foreground.String()),
					"Background": booklit.String(chunk.Style.Background.String()),
				},
			})
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
	var obj bass.Object
	if err := val.Decode(&obj); err == nil {
		return plugin.renderObject(obj)
	}

	var list bass.List
	if err := val.Decode(&list); err == nil {
		return plugin.renderList(list)
	}

	var wlp bass.WorkloadPath
	if err := val.Decode(&wlp); err == nil {
		return plugin.renderWorkloadPath(wlp)
	}

	return plugin.Bass(booklit.String(fmt.Sprintf("%s", val)))
}

func (plugin *Plugin) renderObject(obj bass.Object) (booklit.Content, error) {
	// handle embedded workload paths
	var wlp bass.WorkloadPath
	if err := obj.Decode(&wlp); err == nil {
		return plugin.renderWorkloadPath(wlp)
	}

	var pairs pairs
	for k, v := range obj {
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

		rows = append(rows, booklit.Sequence{
			keyContent,
			subContent,
		})
	}

	return booklit.Styled{
		Style:   "bass-object",
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
	payload, err := bass.MarshalJSON(wlp.Workload)
	if err != nil {
		return nil, err
	}

	var obj bass.Object
	err = bass.UnmarshalJSON(payload, &obj)
	if err != nil {
		return nil, err
	}

	object, err := plugin.renderObject(obj)
	if err != nil {
		return nil, err
	}

	workload, err := plugin.renderWorkload(wlp.Workload)
	if err != nil {
		return nil, err
	}

	run, err := plugin.renderValue(wlp.Workload.Path.ToValue())
	if err != nil {
		return nil, err
	}

	path, err := plugin.renderValue(wlp.Path.ToValue())
	if err != nil {
		return nil, err
	}

	id, err := wlp.Workload.SHA1()
	if err != nil {
		return nil, err
	}

	plugin.toggleID++

	return booklit.Styled{
		Style:   "bass-workload-path",
		Content: path,
		Partials: booklit.Partials{
			"Workload": workload,
			"Object":   object,
			"Run":      run,
			"ID":       booklit.String(fmt.Sprintf("%s-%d", id, plugin.toggleID)),
		},
	}, nil
}

func (plugin *Plugin) renderWorkload(workload bass.Workload) (booklit.Content, error) {
	invader := invaders.Invader{}

	hash := fnv.New64()
	err := bass.NewEncoder(hash).Encode(workload)
	if err != nil {
		return nil, err
	}

	invader.Set(rand.New(rand.NewSource(int64(hash.Sum64()))))

	avatarSvg := new(bytes.Buffer)
	canvas := svg.New(avatarSvg)

	cellSize := 10
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

	return booklit.Styled{
		Style:   "bass-workload",
		Content: booklit.String(avatarSvg.String()),
	}, nil
}

type kv struct {
	k bass.Keyword
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
