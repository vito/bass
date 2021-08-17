package runtimes

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/bass/zapctx"
	"go.uber.org/zap/zaptest"
)

var TestPlatform = bass.Object{"test": bass.Bool(true)}

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

//go:embed testdata/*.bass
var tests embed.FS

func Suite(t *testing.T, pool *Pool) {
	for _, test := range []struct {
		File   string
		Result bass.Value
	}{
		{
			File:   "testdata/response-exit-code.bass",
			Result: bass.Int(42),
		},
		{
			File:   "testdata/response-file.bass",
			Result: bass.NewList(allJSONValues...),
		},
		{
			File:   "testdata/response-stdout.bass",
			Result: bass.NewList(allJSONValues...),
		},
		{
			File:   "testdata/workload-paths.bass",
			Result: bass.NewList(bass.Int(42), bass.String("hello")),
		},
		{
			File:   "testdata/workload-path-image.bass",
			Result: bass.Int(42),
		},
		{
			File:   "testdata/run-workload-path.bass",
			Result: bass.Int(42),
		},
		{
			File:   "testdata/env.bass",
			Result: bass.Int(42),
		},
		{
			File:   "testdata/workload-path-env.bass",
			Result: bass.Int(42),
		},
		{
			File:   "testdata/dir.bass",
			Result: bass.Int(42),
		},
		{
			File:   "testdata/workload-path-dir.bass",
			Result: bass.Int(42),
		},
		{
			File:   "testdata/mount.bass",
			Result: bass.Int(42),
		},
		{
			File:   "testdata/recursive.bass",
			Result: bass.Int(42),
		},
	} {
		test := test
		t.Run(filepath.Base(test.File), func(t *testing.T) {
			t.Parallel()

			tmp := t.TempDir()

			env := NewEnv(tmp, pool)

			ctx := zapctx.ToContext(context.Background(), zaptest.NewLogger(t))

			_, err := bass.EvalFSFile(ctx, env, tests, "testdata/helpers.bass")
			require.NoError(t, err)

			res, err := bass.EvalFSFile(ctx, env, tests, test.File)
			require.NoError(t, err)
			require.NotNil(t, res)
			Equal(t, test.Result, res)
		})
	}
}
