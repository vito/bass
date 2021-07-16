package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	"github.com/vito/bass/cmd/bass/cli"
)

func TestParseArgs(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	type test struct {
		Argv []string

		File string
		Args []bass.Value
	}

	for _, test := range []test{
		{
			Argv: []string{"file"},
			File: "file",
		},
		{
			Argv: []string{"file", "arg1"},
			File: "file",
			Args: []bass.Value{
				bass.String("arg1"),
			},
		},
		{
			Argv: []string{"file", "arg1", "./file1"},
			File: "file",
			Args: []bass.Value{
				bass.String("arg1"),
				bass.FilePath{
					Path: filepath.Join(cwd, "file1"),
				},
			},
		},
		{
			Argv: []string{"file", "arg1", "./dir1/"},
			File: "file",
			Args: []bass.Value{
				bass.String("arg1"),
				bass.DirectoryPath{
					Path: filepath.Join(cwd, "dir1"),
				},
			},
		},
		{
			Argv: []string{"file", "arg1", "/abs1/"},
			File: "file",
			Args: []bass.Value{
				bass.String("arg1"),
				bass.DirectoryPath{
					Path: "/abs1",
				},
			},
		},
		{
			Argv: []string{"file", "arg1", "."},
			File: "file",
			Args: []bass.Value{
				bass.String("arg1"),
				bass.DirectoryPath{
					Path: cwd,
				},
			},
		},
		{
			Argv: []string{"file", "arg1", cwd},
			File: "file",
			Args: []bass.Value{
				bass.String("arg1"),
				bass.DirectoryPath{
					Path: cwd,
				},
			},
		},
	} {
		t.Run(fmt.Sprintf("%v", test.Argv), func(t *testing.T) {
			file, args, err := cli.ParseArgs(test.Argv)
			require.NoError(t, err)
			require.Equal(t, test.File, file)
			require.Equal(t, test.Args, args)
		})
	}
}
