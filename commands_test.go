package bass_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestCommands(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	for _, test := range []struct {
		File string

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
	} {
		test := test
		t.Run(filepath.Base(test.File), func(t *testing.T) {
			stderrBuf := new(syncBuffer)

			env := bass.NewRuntimeEnv(bass.RuntimeState{
				Stderr: stderrBuf,
			})

			res, err := bass.EvalFile(env, test.File)
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
