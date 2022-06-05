package bass_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/bass/testdata"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/is"
	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
	"go.uber.org/zap/zaptest"
)

func TestBass(t *testing.T) {
	is := is.New(t)

	pool, err := runtimes.NewPool(&bass.Config{})
	is.NoErr(err)

	for _, test := range []struct {
		File     string
		Result   bass.Value
		Bindings bass.Bindings
	}{
		{
			File:   "run.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			File:   "load.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			File:   "use.bass",
			Result: bass.String("61,2,3"),
		},
		{
			File:   "env.bass",
			Result: bass.NewList(bass.String("123"), bass.String("123")),
		},
	} {
		test := test
		t.Run(filepath.Base(test.File), func(t *testing.T) {
			is := is.New(t)

			t.Parallel()

			res, err := RunTest(context.Background(), t, pool, test.File, nil)
			is.NoErr(err)
			is.True(res != nil)
			Equal(t, res, test.Result)
		})
	}
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

	source := bass.NewFSPath(testdata.FSID, testdata.FS, bass.ParseFileOrDirPath(file))

	res, err := bass.EvalFSFile(ctx, scope, source)
	if err != nil {
		vtx.Done(err)
		return nil, err
	}

	vtx.Done(nil)

	return res, nil
}

func TestInternalBindings(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name:   "time-measure",
			Scope:  bass.NewEmptyScope(bass.Internal, bass.Ground),
			Bass:   `(time-measure (dump 42))`,
			Result: bass.Int(42),
			Log:    []string{`DEBUG\t\(time \(dump 42\)\) => 42 took \d.+s`},
		},
	} {
		t.Run(example.Name, example.Run)
	}
}
