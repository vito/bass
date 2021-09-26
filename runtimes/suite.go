package runtimes

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/runtimes/testdata"
	"github.com/vito/bass/zapctx"
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
		File   string
		Result bass.Value
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
			File:   "workload-paths.bass",
			Result: bass.NewList(bass.Int(42), bass.String("hello")),
		},
		{
			File:   "workload-path-image.bass",
			Result: bass.Int(42),
		},
		{
			File:   "run-workload-path.bass",
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
			File:   "workload-path-env.bass",
			Result: bass.Int(42),
		},
		{
			File:   "dir.bass",
			Result: bass.Int(42),
		},
		{
			File:   "workload-path-dir.bass",
			Result: bass.Int(42),
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
			File:   "mount-local.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Symbol("eof")),
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
	} {
		test := test
		t.Run(filepath.Base(test.File), func(t *testing.T) {
			t.Parallel()

			res, err := runTest(context.Background(), t, pool, test.File)
			require.NoError(t, err)
			require.NotNil(t, res)
			Equal(t, test.Result, res)
		})
	}

	t.Run("interruptable", func(t *testing.T) {
		timeout := time.Second

		start := time.Now()
		deadline := start.Add(timeout)

		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		_, err := runTest(ctx, t, pool, "sleep.bass")
		require.Error(t, context.Canceled, err)

		require.WithinDuration(t, deadline, time.Now(), time.Second)
	})
}

func runTest(ctx context.Context, t *testing.T, pool *Pool, file string) (bass.Value, error) {
	ctx = zapctx.ToContext(ctx, zaptest.NewLogger(t))

	trace := &bass.Trace{}
	ctx = bass.WithTrace(ctx, trace)
	ctx = bass.WithRuntime(ctx, pool)

	ctx = ioctx.StderrToContext(ctx, os.Stderr)

	scope := NewScope(bass.NewStandardScope(), RunState{
		Dir: bass.NewFSDir(testdata.FS),
	})

	res, err := bass.EvalFSFile(ctx, scope, testdata.FS, file)
	if err != nil {
		bass.WriteError(ctx, err)
		return nil, err
	}

	return res, nil
}
