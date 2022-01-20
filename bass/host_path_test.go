package bass_test

import (
	"testing"

	"github.com/vito/bass/bass"
	"github.com/vito/is"
)

func TestHostPathName(t *testing.T) {
	is := is.New(t)

	is.Equal(
		"dir",
		bass.HostPath{
			ContextDir: "/some/dir",
			Path:       bass.ParseFileOrDirPath("./"),
		}.Name(),
	)

	is.Equal(
		"dir",
		bass.HostPath{
			ContextDir: "/some/dir",
			Path:       bass.ParseFileOrDirPath("."),
		}.Name(),
	)

	is.Equal(
		"dir",
		bass.HostPath{
			ContextDir: "/some/dir",
			Path:       bass.ParseFileOrDirPath("/"),
		}.Name(),
	)

	is.Equal(
		"foo",
		bass.HostPath{
			ContextDir: "/some/dir",
			Path:       bass.ParseFileOrDirPath("./foo/"),
		}.Name(),
	)

	is.Equal(
		"bar",
		bass.HostPath{
			ContextDir: "/some/dir",
			Path:       bass.ParseFileOrDirPath("./foo/bar"),
		}.Name(),
	)
}
