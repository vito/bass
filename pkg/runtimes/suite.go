package runtimes

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
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
	. "github.com/vito/bass/pkg/basstest"
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

func Suite(t *testing.T, pool bass.RuntimePool) {
	for _, test := range []struct {
		File     string
		Result   bass.Value
		Bindings bass.Bindings
		ErrCause string
	}{
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
				bass.Bindings{"a": bass.Int(1)}.Scope(),
				bass.Bindings{"b": bass.Int(2)}.Scope(),
				bass.Bindings{"c": bass.Int(3)}.Scope(),
				bass.Symbol("eof"),
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
			Result: bass.NewList(bass.String("Hello"), bass.String("from"), bass.String("Docker!")),
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
	} {
		test := test
		t.Run(filepath.Base(test.File), func(t *testing.T) {
			is := is.New(t)
			t.Parallel()

			ctx := context.Background()

			// set a reasonable timeout so we get a more descriptive failure than the
			// global go test timeout
			//
			// ideally this would be even lower but we should account for slow
			// networks for image fetching/etc.
			ctx, stop := context.WithTimeout(ctx, 5*time.Minute)
			defer stop()

			displayBuf := new(bytes.Buffer)
			ctx = bass.WithTrace(ctx, &bass.Trace{})
			ctx = ioctx.StderrToContext(ctx, displayBuf)
			res, err := RunTest(ctx, t, pool, test.File, nil)
			t.Logf("progress:\n%s", displayBuf.String())
			if test.ErrCause != "" {
				is.True(err != nil)
				t.Logf("error: %s", err)
				// NB: assert against the root cause of the error, not just Contains
				is.True(strings.HasSuffix(err.Error(), test.ErrCause))
			} else {
				is.NoErr(err)
				is.True(res != nil)
				Equal(t, res, test.Result)
			}
		})
	}

	t.Run("interruptable", func(t *testing.T) {
		is := is.New(t)
		t.Parallel()

		timeout := time.Second

		start := time.Now()
		deadline := start.Add(timeout)

		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		_, err := RunTest(ctx, t, pool, "sleep.bass", nil)
		is.True(errors.Is(err, bass.ErrInterrupted))

		is.True(cmp.Equal(deadline, time.Now(), cmpopts.EquateApproxTime(10*time.Second)))
	})

	t.Run("secrets", func(t *testing.T) {
		t.Parallel()

		is := is.New(t)

		secret := "im-always-angry"

		secrets := bass.Bindings{
			"SECRET": bass.String(secret),
		}.Scope()

		displayBuf := new(bytes.Buffer)
		ctx := context.Background()
		ctx = ioctx.StderrToContext(ctx, displayBuf)
		res, err := RunTest(ctx, t, pool, "secrets.bass", secrets)
		t.Logf("progress:\n%s", displayBuf.String())
		is.NoErr(err)

		var scp *bass.Scope
		err = res.Decode(&scp)
		is.NoErr(err)

		var results []bass.Value
		err = scp.GetDecode("results", &results)
		is.NoErr(err)
		is.True(len(results) > 0)
		for _, r := range results {
			is.Equal(bass.String(secret), r)
		}

		var thunks []bass.Thunk
		err = scp.GetDecode("thunks", &thunks)
		is.NoErr(err)
		for _, thunk := range thunks {
			runtime, err := pool.Select(*thunk.Platform())
			is.NoErr(err)

			buf := new(bytes.Buffer)
			err = runtime.Export(ctx, buf, thunk)
			is.NoErr(err)

			t.Logf("scanning thunk layers")
			is.NoErr(scan(t, buf, secret))

			t.Logf("scanning output displayed to user")
			is.NoErr(scan(t, displayBuf, secret))
		}
	})
}

func RunTest(ctx context.Context, t *testing.T, pool bass.RuntimePool, file string, env *bass.Scope) (bass.Value, error) {
	is := is.New(t)

	ctx = zapctx.ToContext(ctx, zaptest.NewLogger(t))

	r, w := progrock.Pipe()
	recorder := progrock.NewRecorder(w)
	defer recorder.Stop()

	ctx, stop := context.WithCancel(ctx)
	ctx = progrock.RecorderToContext(ctx, recorder)
	recorder.Display(stop, ui.Default, ioctx.StderrFromContext(ctx), r, false)

	trace := &bass.Trace{}
	ctx = bass.WithTrace(ctx, trace)
	ctx = bass.WithRuntimePool(ctx, pool)

	dir, err := filepath.Abs(filepath.Dir(filepath.Join("testdata", file)))
	is.NoErr(err)

	vtx := recorder.Vertex("test", "bass "+file)

	scope := bass.NewRunScope(bass.NewStandardScope(), bass.RunState{
		Dir:    bass.NewHostDir(dir),
		Env:    env,
		Stdin:  bass.NewSource(bass.NewInMemorySource()),
		Stdout: bass.NewSink(bass.NewJSONSink("stdout", vtx.Stdout())),
	})

	scope.Set("*memos*", bass.NewHostPath("./testdata/", bass.ParseFileOrDirPath("bass.lock")))

	source := bass.NewFSPath(testdata.FS, bass.ParseFileOrDirPath(file))

	res, err := bass.EvalFSFile(ctx, scope, source)
	if err != nil {
		vtx.Done(err)
		return nil, err
	}

	vtx.Done(nil)

	return res, nil
}

func scan(t *testing.T, r io.Reader, needle string) error {
	is := is.New(t)

	buf := new(bytes.Buffer)

	_, err := io.Copy(buf, r)
	is.NoErr(err)

unwrap:
	for {
		ct := http.DetectContentType(buf.Bytes())
		switch ct {
		case "application/x-gzip":
			gr, err := gzip.NewReader(buf)
			is.NoErr(err)

			uncompressed := new(bytes.Buffer)
			_, err = io.Copy(uncompressed, gr)
			is.NoErr(err)

			buf = uncompressed

			continue unwrap
		case "application/octet-stream":
			err := scanTar(t, buf, needle)
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

	if strings.Contains(buf.String(), needle) {
		return fmt.Errorf("detected %q", needle)
	}

	return nil
}

func scanTar(t *testing.T, r io.Reader, needle string) error {
	is := is.New(t)

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
		is.NoErr(err)

		err = scan(t, buf, needle)
		if err != nil {
			return fmt.Errorf("scan %s: %w", hdr.Name, err)
		}
	}

	return nil
}
