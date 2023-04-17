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
	err := bass.NewDirPath("foo").Decode(&foo)
	is.NoErr(err)
	is.Equal(foo, bass.NewDirPath("foo"))

	err = bass.NewDirPath("bar").Decode(&foo)
	is.NoErr(err)
	is.Equal(foo, bass.NewDirPath("bar"))

	var path_ bass.Path
	err = bass.NewDirPath("bar").Decode(&path_)
	is.NoErr(err)
	is.Equal(path_, bass.NewDirPath("bar"))

	var comb bass.Combiner
	err = bass.NewDirPath("foo").Decode(&comb)
	is.NoErr(err)
	is.Equal(comb, bass.NewDirPath("foo"))

	var app bass.Applicative
	err = bass.NewDirPath("bar").Decode(&app)
	is.NoErr(err)
	is.Equal(app, bass.NewDirPath("bar"))
}

func TestDirPathEqual(t *testing.T) {
	is := is.New(t)

	Equal(t, bass.NewDirPath("hello"), bass.NewDirPath("hello"))
	Equal(t, bass.NewDirPath(""), bass.NewDirPath(""))
	is.True(!bass.NewDirPath("hello").Equal(bass.NewDirPath("")))
	is.True(!bass.NewDirPath("hello").Equal(bass.FilePath{"hello"}))
	is.True(!bass.NewDirPath("hello").Equal(bass.CommandPath{"hello"}))
	Equal(t, bass.NewDirPath("hello"), wrappedValue{bass.NewDirPath("hello")})
	is.True(!bass.NewDirPath("hello").Equal(wrappedValue{bass.NewDirPath("")}))
}

func TestDirPathIsDir(t *testing.T) {
	is := is.New(t)

	is.True(bass.NewDirPath("hello").IsDir())
}

func TestDirPathFromSlash(t *testing.T) {
	is := is.New(t)

	is.Equal(filepath.FromSlash("./hello/foo/bar/"), bass.NewDirPath("hello/foo/bar").FromSlash())
	is.Equal(filepath.FromSlash("/hello/foo/bar/"), bass.NewDirPath("/hello/foo/bar").FromSlash())
	is.Equal(filepath.FromSlash("./"), bass.NewDirPath(".").FromSlash())
	is.Equal(filepath.FromSlash("./hello/foo/bar/"), bass.NewDirPath("./hello/foo/bar").FromSlash())
}

func TestDirPathName(t *testing.T) {
	is := is.New(t)

	is.Equal("hello", bass.NewDirPath("foo/hello").Name())
	is.Equal("baz.buzz", bass.NewDirPath("foo/bar/baz.buzz").Name())
}

func TestDirPathExtend(t *testing.T) {
	is := is.New(t)

	var parent, child bass.Path

	parent = bass.NewDirPath("foo")

	child = bass.NewDirPath("bar")
	sub, err := parent.Extend(child)
	is.NoErr(err)
	is.Equal(sub, bass.NewDirPath("foo/bar"))

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
	is.True(!bass.FilePath{"hello"}.Equal(bass.NewDirPath("hello")))
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

	child = bass.NewDirPath("bar")
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
	is.True(!bass.CommandPath{"hello"}.Equal(bass.NewDirPath("hello")))
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

	child = bass.NewDirPath("bar")
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
