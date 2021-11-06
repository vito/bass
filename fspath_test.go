package bass_test

import (
	"testing"

	"github.com/matryer/is"
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
			is := is.New(t)

			path, err := bass.ParseFilesystemPath(test.Arg)
			is.NoErr(err)
			is.Equal(path, test.Path)
		})
	}
}

func TestFileOrDirPathFilesystemPath(t *testing.T) {
	is := is.New(t)

	is.Equal(
		bass.DirPath{"dir"},
		bass.FileOrDirPath{
			Dir: &bass.DirPath{"dir"},
		}.FilesystemPath(),
	)

	is.Equal(
		bass.FilePath{"file"},
		bass.FileOrDirPath{
			File: &bass.FilePath{"file"},
		}.FilesystemPath(),
	)
}
