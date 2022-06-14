package bass_test

import (
	"path/filepath"
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
)

func TestDirPathDecode(t *testing.T) {
	is := is.New(t)

	var foo bass.DirPath
	err := bass.DirPath{"foo"}.Decode(&foo)
	is.NoErr(err)
	is.Equal(foo, bass.DirPath{"foo"})

	err = bass.DirPath{"bar"}.Decode(&foo)
	is.NoErr(err)
	is.Equal(foo, bass.DirPath{"bar"})

	var path_ bass.Path
	err = bass.DirPath{"bar"}.Decode(&path_)
	is.NoErr(err)
	is.Equal(path_, bass.DirPath{"bar"})

	var comb bass.Combiner
	err = bass.DirPath{"foo"}.Decode(&comb)
	is.NoErr(err)
	is.Equal(comb, bass.DirPath{"foo"})

	var app bass.Applicative
	err = bass.DirPath{"bar"}.Decode(&app)
	is.NoErr(err)
	is.Equal(app, bass.DirPath{"bar"})
}

func TestDirPathEqual(t *testing.T) {
	is := is.New(t)

	Equal(t, bass.DirPath{"hello"}, bass.DirPath{"hello"})
	Equal(t, bass.DirPath{""}, bass.DirPath{""})
	is.True(!bass.DirPath{"hello"}.Equal(bass.DirPath{""}))
	is.True(!bass.DirPath{"hello"}.Equal(bass.FilePath{"hello"}))
	is.True(!bass.DirPath{"hello"}.Equal(bass.CommandPath{"hello"}))
	Equal(t, bass.DirPath{"hello"}, wrappedValue{bass.DirPath{"hello"}})
	is.True(!bass.DirPath{"hello"}.Equal(wrappedValue{bass.DirPath{""}}))
}

func TestDirPathIsDir(t *testing.T) {
	is := is.New(t)

	is.True(bass.DirPath{"hello"}.IsDir())
}

func TestDirPathFromSlash(t *testing.T) {
	is := is.New(t)

	is.Equal(filepath.FromSlash("./hello/foo/bar/"), bass.DirPath{"hello/foo/bar"}.FromSlash())
	is.Equal(filepath.FromSlash("/hello/foo/bar/"), bass.DirPath{"/hello/foo/bar"}.FromSlash())
	is.Equal(filepath.FromSlash("./"), bass.DirPath{"."}.FromSlash())
	is.Equal(filepath.FromSlash("./hello/foo/bar/"), bass.DirPath{"./hello/foo/bar"}.FromSlash())
}

func TestDirPathName(t *testing.T) {
	is := is.New(t)

	is.Equal("hello", bass.DirPath{"foo/hello"}.Name())
	is.Equal("baz.buzz", bass.DirPath{"foo/bar/baz.buzz"}.Name())
}

func TestDirPathExtend(t *testing.T) {
	is := is.New(t)

	var parent, child bass.Path

	parent = bass.DirPath{"foo"}

	child = bass.DirPath{"bar"}
	sub, err := parent.Extend(child)
	is.NoErr(err)
	is.Equal(sub, bass.DirPath{"foo/bar"})

	child = bass.FilePath{"bar"}
	sub, err = parent.Extend(child)
	is.NoErr(err)
	is.Equal(sub, bass.FilePath{"foo/bar"})

	child = bass.CommandPath{"bar"}
	sub, err = parent.Extend(child)
	is.True(sub == nil)
	is.True(err != nil)
}

func TestFilePathDecode(t *testing.T) {
	is := is.New(t)

	var foo bass.FilePath
	err := bass.FilePath{"foo"}.Decode(&foo)
	is.NoErr(err)
	is.Equal(foo, bass.FilePath{"foo"})

	err = bass.FilePath{"bar"}.Decode(&foo)
	is.NoErr(err)
	is.Equal(foo, bass.FilePath{"bar"})

	var path_ bass.Path
	err = bass.FilePath{"bar"}.Decode(&path_)
	is.NoErr(err)
	is.Equal(path_, bass.FilePath{"bar"})

	var comb bass.Combiner
	err = bass.FilePath{"foo"}.Decode(&comb)
	is.NoErr(err)
	is.Equal(comb, bass.FilePath{"foo"})

	var app bass.Applicative
	err = bass.FilePath{"foo"}.Decode(&app)
	is.NoErr(err)
	is.Equal(comb, bass.FilePath{"foo"})
}

func TestFilePathFromSlash(t *testing.T) {
	is := is.New(t)

	is.Equal(filepath.FromSlash("./hello/foo/bar"), bass.FilePath{"hello/foo/bar"}.FromSlash())
	is.Equal(filepath.FromSlash("./hello/foo/bar"), bass.FilePath{"./hello/foo/bar"}.FromSlash())
	is.Equal(filepath.FromSlash("/hello/foo/bar"), bass.FilePath{"/hello/foo/bar"}.FromSlash())
}

func TestFilePathEqual(t *testing.T) {
	is := is.New(t)

	Equal(t, bass.FilePath{"hello"}, bass.FilePath{"hello"})
	Equal(t, bass.FilePath{""}, bass.FilePath{""})
	is.True(!bass.FilePath{"hello"}.Equal(bass.FilePath{""}))
	is.True(!bass.FilePath{"hello"}.Equal(bass.DirPath{"hello"}))
	is.True(!bass.FilePath{"hello"}.Equal(bass.CommandPath{"hello"}))
	Equal(t, bass.FilePath{"hello"}, wrappedValue{bass.FilePath{"hello"}})
	is.True(!bass.FilePath{"hello"}.Equal(wrappedValue{bass.FilePath{""}}))
}

func TestFilePathIsDir(t *testing.T) {
	is := is.New(t)

	is.True(!bass.FilePath{"hello"}.IsDir())
}

func TestFilePathName(t *testing.T) {
	is := is.New(t)

	is.Equal("hello", bass.FilePath{"foo/hello"}.Name())
	is.Equal("baz.buzz", bass.FilePath{"foo/bar/baz.buzz"}.Name())
}

func TestFilePathExtend(t *testing.T) {
	is := is.New(t)

	var parent, child bass.Path

	parent = bass.FilePath{"foo"}

	child = bass.DirPath{"bar"}
	_, err := parent.Extend(child)
	is.True(err != nil)

	child = bass.FilePath{"bar"}
	_, err = parent.Extend(child)
	is.True(err != nil)

	child = bass.CommandPath{"bar"}
	_, err = parent.Extend(child)
	is.True(err != nil)
}

func TestCommandPathDecode(t *testing.T) {
	is := is.New(t)

	var foo bass.CommandPath
	err := bass.CommandPath{"foo"}.Decode(&foo)
	is.NoErr(err)
	is.Equal(foo, bass.CommandPath{"foo"})

	err = bass.CommandPath{"bar"}.Decode(&foo)
	is.NoErr(err)
	is.Equal(foo, bass.CommandPath{"bar"})

	var path_ bass.Path
	err = bass.CommandPath{"bar"}.Decode(&path_)
	is.NoErr(err)
	is.Equal(path_, bass.CommandPath{"bar"})

	var comb bass.Combiner
	err = bass.CommandPath{"foo"}.Decode(&comb)
	is.NoErr(err)
	is.Equal(comb, bass.CommandPath{"foo"})

	var app bass.Applicative
	err = bass.CommandPath{"foo"}.Decode(&app)
	is.NoErr(err)
	is.Equal(comb, bass.CommandPath{"foo"})
}

func TestCommandPathEqual(t *testing.T) {
	is := is.New(t)

	Equal(t, bass.CommandPath{"hello"}, bass.CommandPath{"hello"})
	Equal(t, bass.CommandPath{""}, bass.CommandPath{""})
	is.True(!bass.CommandPath{"hello"}.Equal(bass.CommandPath{""}))
	is.True(!bass.CommandPath{"hello"}.Equal(bass.DirPath{"hello"}))
	is.True(!bass.CommandPath{"hello"}.Equal(bass.FilePath{"hello"}))
	Equal(t, bass.CommandPath{"hello"}, wrappedValue{bass.CommandPath{"hello"}})
	is.True(!bass.CommandPath{"hello"}.Equal(wrappedValue{bass.CommandPath{""}}))
}

func TestCommandPathName(t *testing.T) {
	is := is.New(t)

	is.Equal("hello", bass.CommandPath{"hello"}.Name())
}

func TestCommandPathExtend(t *testing.T) {
	is := is.New(t)

	var parent, child bass.Path

	parent = bass.CommandPath{"foo"}

	child = bass.DirPath{"bar"}
	_, err := parent.Extend(child)
	is.True(err != nil)

	child = bass.FilePath{"bar"}
	_, err = parent.Extend(child)
	is.True(err != nil)

	child = bass.CommandPath{"bar"}
	_, err = parent.Extend(child)
	is.True(err != nil)
}

func TestExtendPathDecode(t *testing.T) {
	is := is.New(t)

	var foo bass.ExtendPath
	err := bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}.Decode(&foo)
	is.NoErr(err)
	is.Equal(foo, bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	})

	err = bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"baz"},
	}.Decode(&foo)
	is.NoErr(err)
	is.Equal(foo, bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"baz"},
	})

	var path_ bass.Path
	err = bass.ExtendPath{Parent: bass.Symbol("foo"), Child: bass.FilePath{"bar"}}.Decode(&path_)
	is.True(err != nil)

	var comb bass.Combiner
	err = bass.ExtendPath{Parent: bass.Symbol("foo"), Child: bass.FilePath{"bar"}}.Decode(&comb)
	is.True(err != nil)
}

func TestExtendPathEqual(t *testing.T) {
	is := is.New(t)

	Equal(t, bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}, bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	})

	is.True(!bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}.Equal(bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"baz"},
	}))

	Equal(t, bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}, wrappedValue{bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}})

	is.True(!bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}.Equal(wrappedValue{bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"baz"},
	}}))

}
