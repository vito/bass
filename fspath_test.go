package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestParseFilesystemPath(t *testing.T) {
	type test struct {
		Arg  string
		Path bass.FilesystemPath
	}

	for _, test := range []test{
		{
			Arg:  ".",
			Path: bass.DirPath{"."},
		},
		{
			Arg:  "/",
			Path: bass.DirPath{""},
		},
		{
			Arg:  "./",
			Path: bass.DirPath{"."},
		},
		{
			Arg:  "./foo",
			Path: bass.FilePath{"foo"},
		},
		{
			Arg:  "foo",
			Path: bass.FilePath{"foo"},
		},
		{
			Arg:  "./foo/",
			Path: bass.DirPath{"foo"},
		},
		{
			Arg:  "foo/",
			Path: bass.DirPath{"foo"},
		},
		{
			Arg:  "./foo/bar/",
			Path: bass.DirPath{"foo/bar"},
		},
		{
			Arg:  "foo/bar",
			Path: bass.FilePath{"foo/bar"},
		},
	} {
		t.Run(test.Arg, func(t *testing.T) {
			path, err := bass.ParseFilesystemPath(test.Arg)
			require.NoError(t, err)
			require.Equal(t, test.Path, path)
		})
	}
}

func TestFileOrDirPathFilesystemPath(t *testing.T) {
	require.Equal(t,
		bass.FileOrDirPath{
			Dir: &bass.DirPath{"dir"},
		}.FilesystemPath(),
		bass.DirPath{"dir"},
	)

	require.Equal(t,
		bass.FileOrDirPath{
			File: &bass.FilePath{"file"},
		}.FilesystemPath(),
		bass.FilePath{"file"},
	)
}
