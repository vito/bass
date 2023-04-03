package runtimes

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstest"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/runtimes/testdata"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/is"
	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
	"go.uber.org/zap/zaptest"
)

var allJSONValues = []bass.Value{
	bass.Null{},
	bass.Bool(false),
	bass.Bool(true),
	bass.Int(42),
	bass.String("hello"),
	bass.Empty{},
	bass.NewList(bass.Int(0), bass.String("one"), bass.Int(-2)),
	bass.NewEmptyScope(),
	bass.Bindings{"foo": bass.String("bar")}.Scope(),
}

type SuiteTest struct {
	File     string
	Result   bass.Value
	Bindings bass.Bindings
	Timeout  time.Duration
	ErrCause string
}

//go:embed testdata/write.bass
var writeTestContent string

func Suite(t *testing.T, config bass.RuntimeConfig) {
	ctx := context.Background()

	pool, err := NewPool(ctx, &bass.Config{
		Runtimes: []bass.RuntimeConfig{
			config,
		},
	})
	is.New(t).NoErr(err)

	t.Cleanup(func() {
		err := pool.Close()
		if err != nil && !strings.Contains(err.Error(), "context canceled") {
			t.Logf("close pool: %s", err)
		}
	})

	ctx = bass.WithRuntimePool(ctx, pool)

	for _, test := range []SuiteTest{
		{
			File:     "error.bass",
			ErrCause: "42",
		},
		{
			File:   "response-file.bass",
			Result: bass.NewList(allJSONValues...),
		},
		{
			File:   "response-stdout.bass",
			Result: bass.NewList(allJSONValues...),
		},
		{
			File:   "thunk-paths.bass",
			Result: bass.NewList(bass.Int(42), bass.String("hello")),
		},
		{
			File:   "thunk-path-image.bass",
			Result: bass.Int(42),
		},
		{
			File:   "run-thunk-path.bass",
			Result: bass.Int(42),
		},
		{
			File:   "env.bass",
			Result: bass.Int(42),
		},
		{
			File:   "args.bass",
			Result: bass.NewList(bass.String("hello"), bass.String("world")),
		},
		{
			File:   "multi-env.bass",
			Result: bass.NewList(bass.Int(42), bass.Int(21)),
		},
		{
			File:   "thunk-path-env.bass",
			Result: bass.Int(42),
		},
		{
			File:   "dir.bass",
			Result: bass.Int(42),
		},
		{
			File:   "thunk-path-dir.bass",
			Result: bass.Int(42),
		},
		{
			File:   "thunk-path-dir-thunk-path-inputs.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			File:   "mount.bass",
			Result: bass.Int(42),
		},
		{
			File:   "mount-run-dir.bass",
			Result: bass.Int(42),
		},
		{
			File:   "recursive.bass",
			Result: bass.Int(42),
		},
		{
			File: "load.bass",
			Result: bass.NewList(
				bass.String("a!b!c"),
				bass.NewList(
					bass.Bindings{"a": bass.Int(1)}.Scope(),
					bass.Bindings{"b": bass.Int(2)}.Scope(),
					bass.Bindings{"c": bass.Int(3)}.Scope(),
				),
			),
		},
		{
			File:   "host-paths.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			File:   "fs-paths.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			File:   "host-paths-sparse.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3), bass.Int(3)),
		},
		{
			File:   "cache-paths.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			File: "cache-sync.bass",
			Result: bass.NewList(
				bass.NewList(bass.String("1")),
				bass.NewList(bass.String("2")),
				bass.NewList(bass.String("3")),
				bass.NewList(bass.String("4")),
				bass.NewList(bass.String("5")),
				bass.NewList(bass.String("6")),
				bass.NewList(bass.String("7")),
				bass.NewList(bass.String("8")),
				bass.NewList(bass.String("9")),
				bass.NewList(bass.String("10")),
			),
		},
		{
			File:   "cache-cmd.bass",
			Result: bass.String("hello, world!\n"),
		},
		{
			File:   "read-path.bass",
			Result: bass.String("hello, world!\n"),
		},
		{
			File:   "succeeds.bass",
			Result: bass.NewList(bass.Bool(false), bass.Bool(true), bass.Bool(false)),
		},
		{
			File: "many-layers-workdir.bass",
			Result: bass.NewList(
				bass.Int(1),
				bass.Int(2),
				bass.Int(3),
				bass.Int(4),
				bass.Int(5),
				bass.Int(6),
				bass.Int(7),
				bass.Int(8),
				bass.Int(9),
				bass.Int(10),
			),
		},
		{
			File:   "oci-archive-image.bass",
			Result: bass.String("/go"),
		},
		{
			File:   "remount-workdir.bass",
			Result: bass.String("bar\nbaz\nfoo\n"),
		},
		{
			File: "remount-workdir-subdir.bass",
			Result: bass.NewList(
				bass.String("bar\nbaz\nfoo\n"),
				bass.String("foo\n"),
				bass.String("bar\n"),
				bass.String("baz\n"),
			),
		},
		{
			File: "timestamps.bass",
			Result: bass.NewList(
				bass.NewList(bass.String("499162500"), bass.String("499162500")),
				bass.NewList(bass.String("499162500"), bass.String("499162500")),
			),
		},
		{
			File:   "concat.bass",
			Result: bass.String("hello, world!\n"),
		},
		{
			File:   "addrs.bass",
			Result: bass.String("hello, world!"),
		},
		{
			File:   "tls.bass",
			Result: bass.Bool(true),
		},
		{
			File:   "secrets.bass",
			Result: bass.Null{},
			Bindings: bass.Bindings{
				"assert-export-does-not-contain-secret": bass.Func("assert-export-does-not-contain-secret", "[thunk]", func(ctx context.Context, thunk bass.Thunk) error {
					pool, err := bass.RuntimePoolFromContext(ctx)
					if err != nil {
						return err
					}

					runtime, err := pool.Select(*thunk.Platform())
					if err != nil {
						return err
					}

					buf := new(bytes.Buffer)
					err = runtime.Export(ctx, buf, thunk)
					if err != nil {
						return err
					}

					return detectSecret(buf, "hunter2")
				}),
				"assert-does-not-contain-secret": bass.Func("assert-does-not-contain-secret", "[display]", func(display string) error {
					return detectSecret(bytes.NewBufferString(display), "hunter2")
				}),
			},
		},
		{
			File:     "sleep.bass",
			Timeout:  time.Second,
			ErrCause: bass.ErrInterrupted.Error(),
		},
		{
			File:   "export.bass",
			Result: bass.Null{},
		},
		{
			File: "write.bass",
			Bindings: bass.Bindings{
				"*tmp*": bass.NewHostDir(t.TempDir()),
			},
			Result: bass.NewList(
				bass.String(writeTestContent),
				bass.String("Hello, world!\n"),
			),
		},
		// TODO: test publishing somehow :/
		{
			File: "docker-build.bass",
			Result: bass.NewList(
				bass.String("hello from Dockerfile\n"),
				bass.String("hello from Dockerfile.alt\n"),
				bass.String("hello from alt stage in Dockerfile\n"),
				bass.String("hello from Dockerfile with message sup\n"),
				bass.String("hello from Dockerfile with env bar\nbar\n"),
			),
		},
		{
			File: "entrypoints.bass",
			Result: bass.Bindings{
				"from-image": bass.String("git version 2.36.3\n"),
				"from-thunk": bass.String(
					"setting entrypoint\n" +
						"using entrypoint\n" +
						"using entrypoint again\n" +
						"removing entrypoint\n" +
						"no more entrypoint\n",
				),
			}.Scope(),
		},
	} {
		test := test
		t.Run(filepath.Base(test.File), func(t *testing.T) {
			is := is.New(t)
			t.Parallel()

			res, err := test.Run(ctx, t, nil)
			if test.ErrCause != "" {
				is.True(err != nil)
				t.Logf("error: %q", err.Error())
				// NB: assert against the root cause of the error, not just Contains
				lines := strings.Split(err.Error(), "\n")
				is.True(strings.HasSuffix(lines[0], test.ErrCause))
			} else {
				is.NoErr(err)
				is.True(res != nil)
				basstest.Equal(t, res, test.Result)
			}
		})
	}
}

func (test SuiteTest) Run(ctx context.Context, t *testing.T, env *bass.Scope) (val bass.Value, err error) {
	is := is.New(t)

	ctx = zapctx.ToContext(ctx, zaptest.NewLogger(t))

	r, w := progrock.Pipe()
	recorder := progrock.NewRecorder(w)
	defer recorder.Stop()

	displayBuf := new(bytes.Buffer)
	ctx = ioctx.StderrToContext(ctx, displayBuf)
	defer func() {
		if err != nil {
			cli.WriteError(ctx, err)
		}

		t.Logf("progress:\n%s", displayBuf.String())
	}()

	ctx, stop := context.WithCancel(ctx)
	ctx = progrock.RecorderToContext(ctx, recorder)
	recorder.Display(stop, ui.Default, ioctx.StderrFromContext(ctx), r, false)

	trace := &bass.Trace{}
	ctx = bass.WithTrace(ctx, trace)

	dir, err := filepath.Abs(filepath.Dir(filepath.Join("testdata", test.File)))
	is.NoErr(err)

	vtx := recorder.Vertex("test", "bass "+test.File)

	var scope *bass.Scope
	if test.Bindings != nil {
		scope = bass.NewScope(test.Bindings, bass.Ground)
	} else {
		scope = bass.NewStandardScope()
	}
	scope = bass.NewRunScope(scope, bass.RunState{
		Dir:    bass.NewHostDir(dir),
		Env:    env,
		Stdin:  bass.NewSource(bass.NewInMemorySource()),
		Stdout: bass.NewSink(bass.NewJSONSink("stdout", vtx.Stdout())),
	})
	scope.Set("*memos*", bass.NewHostPath("./testdata/", bass.ParseFileOrDirPath("bass.lock")))
	scope.Set("*display*", bass.Func("*display*", "[]", func() string {
		return displayBuf.String()
	}))

	source := bass.NewFSPath(testdata.FS, bass.ParseFileOrDirPath(test.File))

	timeout := test.Timeout
	if timeout == 0 {
		// set a reasonable timeout so we get a more descriptive failure than the
		// global go test timeout
		//
		// ideally this would be even lower but we should account for slow
		// networks for image fetching/etc.
		timeout = 5 * time.Minute
	}

	start := time.Now()
	deadline := start.Add(timeout)

	ctx, stop = context.WithDeadline(ctx, deadline)
	defer stop()
	res, err := bass.EvalFSFile(ctx, scope, source)
	if err != nil {
		vtx.Done(err)
		return nil, err
	}

	if test.Timeout != 0 {
		is.True(cmp.Equal(deadline, time.Now(), cmpopts.EquateApproxTime(10*time.Second)))
	}

	vtx.Done(nil)

	return res, nil
}

func detectSecret(r io.Reader, needle string) error {
	buf := new(bytes.Buffer)

	_, err := io.Copy(buf, r)
	if err != nil {
		return err
	}

unwrap:
	for {
		ct := http.DetectContentType(buf.Bytes())
		switch ct {
		case "application/x-gzip":
			gr, err := gzip.NewReader(buf)
			if err != nil {
				return err
			}

			uncompressed := new(bytes.Buffer)
			_, err = io.Copy(uncompressed, gr)
			if err != nil {
				return err
			}

			buf = uncompressed

			continue unwrap
		case "application/octet-stream":
			err := scanTar(buf, needle)
			if err == nil {
				// valid tar archive and nothing detected
				return nil
			} else if err == tar.ErrHeader {
				// not a tar archive; scan raw content
				break unwrap
			} else {
				// scan failed; propagate
				return fmt.Errorf("scan tar: %w", err)
			}
		default:
			break unwrap
		}
	}

	out := buf.String()
	loc := strings.Index(out, needle)
	if loc != -1 {
		before := loc - 10
		if before < 0 {
			before = 0
		}
		return fmt.Errorf("detected %q: ...%s", needle, out[before:loc+len(needle)])
	}

	return nil
}

func scanTar(r io.Reader, needle string) error {
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			// not a tar archive
			return err
		}

		buf := new(bytes.Buffer)

		_, err = io.Copy(buf, tr)
		if err != nil {
			return err
		}

		err = detectSecret(buf, needle)
		if err != nil {
			return fmt.Errorf("scan %s: %w", hdr.Name, err)
		}
	}

	return nil
}
