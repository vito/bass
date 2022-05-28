package bass_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
			Attempted:  filepath.Join(tmp, "file"),
		}, err)
	})

	t.Run("can read . context dir", func(t *testing.T) {
		is := is.New(t)

		// hack: open the current test
		_, f, _, _ := runtime.Caller(0)
		escape := bass.ParseFileOrDirPath(filepath.Base(f))

		hp := bass.NewHostPath(".", escape)

		cachePath, err := hp.CachePath(ctx, bass.CacheHome)
		is.NoErr(err)
		abs, err := filepath.Abs(cachePath)
		is.NoErr(err)
		is.Equal(f, abs)
	})

	t.Run("cannot escape . context dir to file", func(t *testing.T) {
		is := is.New(t)

		escape := bass.ParseFileOrDirPath("../file")
		hp := bass.NewHostPath(".", escape)

		_, err := hp.CachePath(ctx, bass.CacheHome)
		is.Equal(bass.HostPathEscapeError{
			ContextDir: ".",
			Attempted:  "../file",
		}, err)
	})

	t.Run("cannot escape . context dir to parent dir", func(t *testing.T) {
		is := is.New(t)

		escape := bass.ParseFileOrDirPath("../")
		hp := bass.NewHostPath(".", escape)

		_, err := hp.CachePath(ctx, bass.CacheHome)
		is.Equal(bass.HostPathEscapeError{
			ContextDir: ".",
			Attempted:  "..",
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
			Attempted:  filepath.Join(tmp, "file"),
		}, err)
	})

	t.Run("can read . context dir", func(t *testing.T) {
		is := is.New(t)

		// hack: open the current test
		_, f, _, _ := runtime.Caller(0)
		escape := bass.ParseFileOrDirPath(filepath.Base(f))

		hp := bass.NewHostPath(".", escape)

		rc, err := hp.Open(ctx)
		is.NoErr(err)

		content, err := io.ReadAll(rc)
		is.NoErr(err)
		is.True(strings.HasPrefix(string(content), "package bass_test\n"))
		is.NoErr(rc.Close())
	})

	t.Run("cannot escape . context dir", func(t *testing.T) {
		is := is.New(t)

		escape := bass.ParseFileOrDirPath("../file")
		hp := bass.NewHostPath(".", escape)

		_, err := hp.Open(ctx)
		is.Equal(bass.HostPathEscapeError{
			ContextDir: ".",
			Attempted:  "../file",
		}, err)
	})
}
