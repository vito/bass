package bass_test

import (
	"bytes"
	"os"
	"path/filepath"
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

		Stderr string
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
			File:   "testdata/commands/stdout-next.bass",
			Result: bass.Int(42),
		},
		{
			File:   "testdata/commands/paths.bass",
			Result: bass.Null{},
			Stderr: filepath.Join(cwd, "foo") + "\n",
		},
	} {
		test := test
		t.Run(test.File, func(t *testing.T) {
			stderrBuf := new(bytes.Buffer)

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
			}
		})
	}
}
