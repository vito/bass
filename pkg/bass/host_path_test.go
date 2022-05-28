package bass_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/vito/bass/pkg/bass"
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

func TestHostPathCachePath(t *testing.T) {
	ctx := context.Background()

	t.Run("can read files", func(t *testing.T) {
		is := is.New(t)

		contextDir := t.TempDir()
		fooPath := filepath.Join(contextDir, "foo")
		is.NoErr(os.WriteFile(fooPath, []byte("hi"), 0644))

		hp := bass.NewHostPath(contextDir, bass.ParseFileOrDirPath("./foo"))

		cachePath, err := hp.CachePath(ctx, bass.CacheHome)
		is.NoErr(err)
		is.Equal(fooPath, cachePath)
	})

	t.Run("cannot escape context dir", func(t *testing.T) {
		is := is.New(t)

		tmp := t.TempDir()
		is.NoErr(os.MkdirAll(filepath.Join(tmp, "ctx"), 0755))
		is.NoErr(os.WriteFile(filepath.Join(tmp, "ctx", "file"), []byte("should-not-open"), 0644))
		is.NoErr(os.WriteFile(filepath.Join(tmp, "file"), []byte("should-not-reach"), 0644))

		escape := bass.ParseFileOrDirPath("../file")
		hp := bass.NewHostPath(filepath.Join(tmp, "ctx"), escape)

		_, err := hp.CachePath(ctx, bass.CacheHome)
		is.Equal(bass.HostPathEscapeError{
			ContextDir: filepath.Join(tmp, "ctx"),
			Attempted:  escape,
		}, err)
	})
}

func TestHostPathOpen(t *testing.T) {
	ctx := context.Background()

	t.Run("can read files", func(t *testing.T) {
		is := is.New(t)

		contextDir := t.TempDir()
		is.NoErr(os.WriteFile(filepath.Join(contextDir, "foo"), []byte("hi"), 0644))

		hp := bass.NewHostPath(contextDir, bass.ParseFileOrDirPath("./foo"))

		rc, err := hp.Open(ctx)
		is.NoErr(err)

		content, err := io.ReadAll(rc)
		is.NoErr(err)
		is.Equal(content, []byte("hi"))

		is.NoErr(rc.Close())
	})

	t.Run("cannot escape context dir", func(t *testing.T) {
		is := is.New(t)

		tmp := t.TempDir()
		is.NoErr(os.MkdirAll(filepath.Join(tmp, "ctx"), 0755))
		is.NoErr(os.WriteFile(filepath.Join(tmp, "ctx", "file"), []byte("should-not-open"), 0644))
		is.NoErr(os.WriteFile(filepath.Join(tmp, "file"), []byte("should-not-reach"), 0644))

		escape := bass.ParseFileOrDirPath("../file")
		hp := bass.NewHostPath(filepath.Join(tmp, "ctx"), escape)

		_, err := hp.Open(ctx)
		is.Equal(bass.HostPathEscapeError{
			ContextDir: filepath.Join(tmp, "ctx"),
			Attempted:  escape,
		}, err)
	})
}
