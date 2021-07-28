package bass_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	"github.com/vito/bass/zapctx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func logger(dest io.Writer) *zap.Logger {
	zapcfg := zap.NewDevelopmentEncoderConfig()
	zapcfg.EncodeLevel = nil
	zapcfg.EncodeTime = nil

	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zapcfg),
		zapcore.AddSync(dest),
		zapcore.DebugLevel,
	))
}

func TestCommands(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	for _, test := range []struct {
		File string
		Args []bass.Value

		Result bass.Value
		ErrMsg string

		Stderr         string
		StderrContains string
	}{
		{
			File:   "testdata/commands/true.bass",
			Result: bass.Null{},
		},
		{
			File:   "testdata/commands/false.bass",
			ErrMsg: "exit status 1",
		},
		{
			File:   "testdata/commands/argv.bass",
			Result: bass.Null{},
			Stderr: "hello, world!\n",
		},
		{
			File:   "testdata/commands/argv-vars.bass",
			Result: bass.Null{},
			Stderr: fmt.Sprintf(
				"hello --foo %s baz buzz lightyear? buzz aldrin!\n",
				filepath.Join(cwd, "bar"),
			),
		},
		{
			File:   "testdata/commands/stdout-next.bass",
			Result: bass.Int(42),
		},
		{
			File:   "testdata/commands/paths.bass",
			Result: bass.Null{},
			Stderr: filepath.Join(cwd, "foo") + "\n",
		},
		{
			File:   "testdata/commands/stdio.bass",
			Result: bass.Null{},
			Stderr: "start\n42\ntrue\n{:a 1}\nsauce\ndone\n",
		},
		{
			File:           "testdata/commands/env.bass",
			Result:         bass.Null{},
			StderrContains: "FOO=hello\n",
		},
		{
			File:           "testdata/commands/env-paths.bass",
			Result:         bass.Null{},
			StderrContains: "HERE=" + filepath.Join(cwd, "here") + "\n",
		},
		{
			File:           "testdata/commands/dir.bass",
			Result:         bass.Null{},
			StderrContains: "./env-paths.bass",
		},
		{
			File:   "testdata/commands/args.bass",
			Args:   []bass.Value{bass.String("arg1"), bass.String("arg2"), bass.FilePath{"foo"}},
			Result: bass.NewList(bass.String("arg1"), bass.String("arg2"), bass.FilePath{"foo"}),
			Stderr: fmt.Sprintf("123 arg1 arg2 %s/foo\n", cwd),
		},
	} {
		test := test
		t.Run(filepath.Base(test.File), func(t *testing.T) {
			stderrBuf := new(syncBuffer)

			env := bass.NewRuntimeEnv(bass.RuntimeState{
				Stderr: stderrBuf,
				Args:   test.Args,
			})

			ctx := zapctx.ToContext(context.Background(), logger(stderrBuf))

			res, err := bass.EvalFile(ctx, env, test.File)
			if test.ErrMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.ErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, res)
				Equal(t, test.Result, res)
			}

			if test.Stderr != "" {
				require.Equal(t, test.Stderr, stderrBuf.String())
			} else if test.StderrContains != "" {
				require.Contains(t, stderrBuf.String(), test.StderrContains)
			}
		})
	}
}

// prevent data races due to concurrent writes from (log ...) and
// `exec.(*Cmd).Start`
type syncBuffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *syncBuffer) Read(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Read(p)
}

func (b *syncBuffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *syncBuffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}
