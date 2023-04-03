package bass_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/bass/testdata"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/is"
	"go.uber.org/zap/zaptest"
)

func TestBassSessionRun(t *testing.T) {
	for _, test := range []sessionTest{
		{
			File: "run.bass",
		},
		{
			File: "load.bass",
		},
		{
			File: "use.bass",
		},
		{
			File: "env.bass",
		},
	} {
		test := test
		t.Run(filepath.Base(test.File), func(t *testing.T) {
			t.Parallel()
			test.Run(t)
		})
	}
}

type sessionTest struct {
	File string
}

func (test sessionTest) Run(t *testing.T) {
	is := is.New(t)

	ctx := context.Background()
	ctx = zapctx.ToContext(ctx, zaptest.NewLogger(t))
	ctx = ioctx.StderrToContext(ctx, os.Stderr)

	err := bass.NewBass().Run(ctx, bass.Thunk{
		Args: []bass.Value{
			bass.NewFSPath(testdata.FS, bass.ParseFileOrDirPath(test.File)),
		},
	}, bass.RunState{})
	is.NoErr(err)
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

func TestSessionClosesSources(t *testing.T) {
	rec := &closeRecorder{
		Reader: bytes.NewBufferString("hello"),
	}

	readable := stubReadable{
		ReadCloser: rec,
	}

	is := is.New(t)

	scope := bass.NewStandardScope()
	scope.Set("opener", readable)

	session := bass.NewSession(scope)

	is.NoErr(session.Run(
		context.Background(),
		bass.Thunk{
			Args: []bass.Value{
				bass.NewFSPath(testdata.FS, bass.ParseFileOrDirPath("read.bass")),
			},
		},
		bass.RunState{},
	))

	is.True(rec.closed)
}

type stubReadable struct {
	bass.Null
	io.ReadCloser
}

func (v stubReadable) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *bass.Readable:
		*x = v
		return nil
	default:
		return v.Null.Decode(dest)
	}
}

func (v stubReadable) CachePath(ctx context.Context, dest string) (string, error) {
	return "", fmt.Errorf("%s.CachePath not implemented", v)
}

// Open opens the resource for reading.
func (v stubReadable) Open(context.Context) (io.ReadCloser, error) {
	return v.ReadCloser, nil
}

type closeRecorder struct {
	io.Reader
	closed bool
}

func (rec *closeRecorder) Close() error {
	rec.closed = true
	return nil
}
