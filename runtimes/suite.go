package runtimes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/runtimes/testdata"
	"github.com/vito/bass/zapctx"
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

func Suite(t *testing.T, pool *Pool) {
	for _, test := range []struct {
		File     string
		Result   bass.Value
		Bindings bass.Bindings
	}{
		{
			File:   "response-exit-code.bass",
			Result: bass.Int(42),
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
				bass.NewList(bass.String("hello"), bass.FilePath{Path: "./goodbye"}),
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
			File:   "host-paths-sparse.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3), bass.Int(3)),
		},
		{
			File:   "cache-paths.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			File:   "read-path.bass",
			Result: bass.String("hello, world!\n"),
		},
	} {
		test := test
		t.Run(filepath.Base(test.File), func(t *testing.T) {
			is := is.New(t)

			t.Parallel()

			res, err := runTest(context.Background(), t, pool, test.File)
			is.NoErr(err)
			is.True(res != nil)
			Equal(t, res, test.Result)
		})
	}

	t.Run("interruptable", func(t *testing.T) {
		is := is.New(t)

		timeout := time.Second

		start := time.Now()
		deadline := start.Add(timeout)

		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		_, err := runTest(ctx, t, pool, "sleep.bass")
		is.True(errors.Is(err, bass.ErrInterrupted))

		is.True(cmp.Equal(deadline, time.Now(), cmpopts.EquateApproxTime(10*time.Second)))
	})
}

func runTest(ctx context.Context, t *testing.T, pool *Pool, file string) (bass.Value, error) {
	is := is.New(t)

	ctx = zapctx.ToContext(ctx, zaptest.NewLogger(t))

	r, w := progrock.Pipe()
	recorder := progrock.NewRecorder(w)
	defer recorder.Stop()

	ctx, stop := context.WithCancel(ctx)
	ctx = progrock.RecorderToContext(ctx, recorder)
	recorder.Display(stop, ui.Default, os.Stderr, r, false)

	trace := &bass.Trace{}
	ctx = bass.WithTrace(ctx, trace)
	ctx = bass.WithRuntime(ctx, pool)

	ctx = ioctx.StderrToContext(ctx, os.Stderr)

	td, err := filepath.Abs("./testdata/")
	is.NoErr(err)

	scope := NewScope(bass.NewStandardScope(), RunState{
		Dir:    bass.NewHostPath(td),
		Args:   bass.Empty{},
		Stdin:  bass.NewSource(bass.NewInMemorySource()),
		Stdout: bass.NewSink(bass.NewInMemorySink()),
	})

	res, err := bass.EvalFSFile(ctx, scope, testdata.FS, file)
	if err != nil {
		bass.WriteError(ctx, err)
		return nil, err
	}

	return res, nil
}
