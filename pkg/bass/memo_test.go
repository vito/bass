package bass_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"testing/fstest"
	"time"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstest"
	"github.com/vito/bass/pkg/proto"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/is"
	"golang.org/x/sync/errgroup"
)

func TestOpenMemosHostPath(t *testing.T) {
	ctx := context.Background()

	t.Run("file exists", func(t *testing.T) {
		is := is.New(t)

		dir := t.TempDir()
		bassLock := filepath.Join(dir, "test.lock")

		// create lock file
		is.NoErr(os.WriteFile(bassLock, nil, 0644))

		fp := bass.NewHostPath(dir, bass.ParseFileOrDirPath("./test.lock"))
		memos, err := bass.OpenMemos(ctx, fp)
		is.NoErr(err)

		testRW(t, memos, bassLock)
	})

	t.Run("file doesn't exist", func(t *testing.T) {
		is := is.New(t)

		dir := t.TempDir()
		bassLock := filepath.Join(dir, "test.lock")

		fp := bass.NewHostPath(dir, bass.ParseFileOrDirPath("./test.lock"))
		memos, err := bass.OpenMemos(ctx, fp)
		is.NoErr(err)

		testRW(t, memos, bassLock)
	})
}

var fakePlatform = bass.Platform{
	OS: "fake",
}

func withFakeRuntime(ctx context.Context, paths []ExportPath) context.Context {
	return bass.WithRuntimePool(ctx, &runtimes.Pool{
		Runtimes: []runtimes.Assoc{
			{
				Platform: fakePlatform,
				Runtime: &FakeRuntime{
					ExportPaths: paths,
				},
			},
		},
	})
}

func genLockfile(t *testing.T, gen func(bass.Memos) error) []byte {
	is := is.New(t)

	dir := t.TempDir()

	genLock := filepath.Join(dir, "test.lock")
	is.NoErr(gen(bass.NewLockfileMemo(genLock)))

	lockContent, err := os.ReadFile(genLock)
	is.NoErr(err)

	return lockContent
}

func uniq(thunk bass.Thunk) bass.Thunk {
	return thunk.WithLabel("now", bass.Int(time.Now().UnixNano()))
}

func TestOpenMemosThunkPath(t *testing.T) {
	baseThunk := bass.Thunk{
		Image: &bass.ThunkImage{
			Ref: &bass.ThunkImageRef{
				Platform: fakePlatform,
			},
		},
		Cmd: bass.ThunkCmd{Cmd: &bass.CommandPath{"foo"}},
	}

	t.Run("file exists", func(t *testing.T) {
		is := is.New(t)

		thunk := uniq(baseThunk)
		thunkProto, err := thunk.Proto()
		is.NoErr(err)

		// able to find lock file
		ctx := withFakeRuntime(context.Background(), []ExportPath{
			{&proto.ThunkPath{
				Thunk: thunkProto,
				Path: &proto.FilesystemPath{
					Path: &proto.FilesystemPath_File{
						File: &proto.FilePath{Path: "foo/named.lock"},
					},
				},
			}, fstest.MapFS{
				"foo/named.lock": {
					Data: genLockfile(t, func(m bass.Memos) error {
						return m.Store(thunkProto, "bnd", bass.String("a"), bass.Int(1))
					}),
					Mode: 0644,
				},
			}},
		})

		memos, err := bass.OpenMemos(ctx, bass.ThunkPath{
			Thunk: thunk,
			Path:  bass.ParseFileOrDirPath("foo/named.lock"),
		})
		is.NoErr(err)

		res, found, err := memos.Retrieve(thunkProto, "bnd", bass.String("a"))
		is.NoErr(err)
		is.True(found)
		basstest.Equal(t, res, bass.Int(1))

		// noop
		err = memos.Store(thunkProto, "bnd", bass.String("b"), bass.Int(2))
		is.NoErr(err)

		// can't find previous writes
		_, found, err = memos.Retrieve(thunkProto, "bnd", bass.String("b"))
		is.NoErr(err)
		is.True(!found)
	})

	t.Run("file doesn't exist", func(t *testing.T) {
		is := is.New(t)
		thunk := uniq(baseThunk)

		// unable to find lock file
		ctx := withFakeRuntime(context.Background(), []ExportPath{})

		_, err := bass.OpenMemos(ctx, bass.ThunkPath{
			Thunk: thunk,
			Path:  bass.ParseFileOrDirPath("foo/named.lock"),
		})
		is.True(err != nil)
	})
}

func TestLockfileMemoConcurrentWrites(t *testing.T) {
	is := is.New(t)

	dir := t.TempDir()

	memos := bass.NewLockfileMemo(filepath.Join(dir, "test.lock"))

	thunk := bass.Thunk{Cmd: bass.ThunkCmd{Cmd: &bass.CommandPath{"foo"}}}
	thunkProto, err := thunk.Proto()
	is.NoErr(err)

	eg := new(errgroup.Group)
	for i := 0; i < 100; i++ {
		num := i

		eg.Go(func() error {
			sym := bass.String(strconv.Itoa(num))
			return memos.Store(thunkProto, "bnd", sym, bass.Int(num))
		})
	}

	is.NoErr(eg.Wait())

	for i := 0; i < 100; i++ {
		sym := bass.String(strconv.Itoa(i))
		val, found, err := memos.Retrieve(thunkProto, "bnd", sym)
		is.NoErr(err)
		is.True(found)
		basstest.Equal(t, val, bass.Int(i))
	}
}

func testRW(t *testing.T, memos bass.Memos, bassLock string) {
	is := is.New(t)

	thunk1 := bass.Thunk{Cmd: bass.ThunkCmd{Cmd: &bass.CommandPath{"foo"}}}
	thunk1P, err := thunk1.Proto()
	is.NoErr(err)
	thunk2 := bass.Thunk{Cmd: bass.ThunkCmd{Cmd: &bass.CommandPath{"bar"}}}
	thunk2P, err := thunk2.Proto()
	is.NoErr(err)

	// no initial value
	_, found, err := memos.Retrieve(thunk1P, "bnd", bass.String("a"))
	is.NoErr(err)
	is.True(!found)

	// set values
	err = memos.Store(thunk1P, "bnd", bass.String("a"), bass.Int(1))
	is.NoErr(err)
	err = memos.Store(thunk1P, "bnd", bass.String("b"), bass.Int(2))
	is.NoErr(err)
	err = memos.Store(thunk2P, "bnd", bass.String("a"), bass.String("one"))
	is.NoErr(err)

	// file now exists
	_, err = os.Stat(bassLock)
	is.NoErr(err)

	// has values
	res, found, err := memos.Retrieve(thunk1P, "bnd", bass.String("a"))
	is.NoErr(err)
	is.True(found)
	basstest.Equal(t, res, bass.Int(1))
	res, found, err = memos.Retrieve(thunk1P, "bnd", bass.String("b"))
	is.NoErr(err)
	is.True(found)
	basstest.Equal(t, res, bass.Int(2))
	res, found, err = memos.Retrieve(thunk2P, "bnd", bass.String("a"))
	is.NoErr(err)
	is.True(found)
	basstest.Equal(t, res, bass.String("one"))

	// remove value
	err = memos.Remove(thunk1P, "bnd", bass.String("a"))
	is.NoErr(err)

	// no longer has value
	_, found, err = memos.Retrieve(thunk1P, "bnd", bass.String("a"))
	is.NoErr(err)
	is.True(!found)

	// still has other values
	res, found, err = memos.Retrieve(thunk1P, "bnd", bass.String("b"))
	is.NoErr(err)
	is.True(found)
	basstest.Equal(t, res, bass.Int(2))
	res, found, err = memos.Retrieve(thunk2P, "bnd", bass.String("a"))
	is.NoErr(err)
	is.True(found)
	basstest.Equal(t, res, bass.String("one"))
}
