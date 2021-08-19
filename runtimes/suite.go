package runtimes

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
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
	bass.Object{},
	bass.Object{"foo": bass.String("bar")},
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
			File:   "recursive.bass",
			Result: bass.Int(42),
		},
	} {
		test := test
		t.Run(filepath.Base(test.File), func(t *testing.T) {
			t.Parallel()

			env := NewEnv(pool)

			ctx := zapctx.ToContext(context.Background(), zaptest.NewLogger(t))

			trace := &bass.Trace{}
			ctx = bass.WithTrace(ctx, trace)

			res, err := bass.EvalFSFile(ctx, env, testdata.FS, test.File)
			if err != nil {
				trace.Write(os.Stderr)
			}

			require.NoError(t, err)
			require.NotNil(t, res)
			Equal(t, test.Result, res)
		})
	}
}
