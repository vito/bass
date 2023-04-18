package bass_test

import (
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/is"
)

func TestParseFilesystemPath(t *testing.T) {
	type test struct {
		Arg  string
		Path bass.FilesystemPath
	}

	for _, test := range []test{
		{
			Arg:  ".",
			Path: bass.DirPath{Path: "."},
		},
		{
			Arg:  "/",
			Path: bass.DirPath{Path: ""},
		},
		{
			Arg:  "./",
			Path: bass.DirPath{Path: "."},
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
			Path: bass.DirPath{Path: "foo"},
		},
		{
			Arg:  "foo/",
			Path: bass.DirPath{Path: "foo"},
		},
		{
			Arg:  "./foo/bar/",
			Path: bass.DirPath{Path: "foo/bar"},
		},
		{
			Arg:  "foo/bar",
			Path: bass.FilePath{"foo/bar"},
		},
	} {
		t.Run(test.Arg, func(t *testing.T) {
			is := is.New(t)

			path := bass.ParseFileOrDirPath(test.Arg).FilesystemPath()
			is.Equal(path, test.Path)
		})
	}
}

func TestFileOrDirPathFilesystemPath(t *testing.T) {
	is := is.New(t)

	is.Equal(
		bass.NewDirPath("dir"),
		bass.FileOrDirPath{
			Dir: &bass.DirPath{Path: "dir"},
		}.FilesystemPath(),
	)

	is.Equal(
		bass.FilePath{"file"},
		bass.FileOrDirPath{
			File: &bass.FilePath{"file"},
		}.FilesystemPath(),
	)
}
