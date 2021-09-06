package bass_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
)

func TestDirPathDecode(t *testing.T) {
	var foo bass.DirPath
	err := bass.DirPath{"foo"}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, bass.DirPath{"foo"}, foo)

	err = bass.DirPath{"bar"}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, bass.DirPath{"bar"}, foo)

	var path_ bass.Path
	err = bass.DirPath{"bar"}.Decode(&path_)
	require.NoError(t, err)
	require.Equal(t, bass.DirPath{"bar"}, path_)

	var comb bass.Combiner
	err = bass.DirPath{"foo"}.Decode(&comb)
	require.Error(t, err)

	var app bass.Applicative
	err = bass.DirPath{"foo"}.Decode(&app)
	require.Error(t, err)
}

func TestDirPathEqual(t *testing.T) {
	require.True(t, bass.DirPath{"hello"}.Equal(bass.DirPath{"hello"}))
	require.True(t, bass.DirPath{""}.Equal(bass.DirPath{""}))
	require.False(t, bass.DirPath{"hello"}.Equal(bass.DirPath{""}))
	require.False(t, bass.DirPath{"hello"}.Equal(bass.FilePath{"hello"}))
	require.False(t, bass.DirPath{"hello"}.Equal(bass.CommandPath{"hello"}))
	require.True(t, bass.DirPath{"hello"}.Equal(wrappedValue{bass.DirPath{"hello"}}))
	require.False(t, bass.DirPath{"hello"}.Equal(wrappedValue{bass.DirPath{""}}))
}

func TestDirPathIsDir(t *testing.T) {
	require.True(t, bass.DirPath{"hello"}.IsDir())
}

func TestDirPathFromSlash(t *testing.T) {
	require.Equal(t,
		bass.DirPath{"hello/foo/bar"}.FromSlash(),
		filepath.FromSlash("./hello/foo/bar/"),
	)

	require.Equal(t,
		bass.DirPath{"/hello/foo/bar"}.FromSlash(),
		filepath.FromSlash("/hello/foo/bar/"),
	)

	require.Equal(t,
		bass.DirPath{"."}.FromSlash(),
		filepath.FromSlash("./"),
	)

	require.Equal(t,
		bass.DirPath{"./hello/foo/bar"}.FromSlash(),
		filepath.FromSlash("./hello/foo/bar/"),
	)
}

func TestDirPathExtend(t *testing.T) {
	var parent, child bass.Path

	parent = bass.DirPath{"foo"}

	child = bass.DirPath{"bar"}
	sub, err := parent.Extend(child)
	require.NoError(t, err)
	require.Equal(t, bass.DirPath{"foo/bar"}, sub)

	child = bass.FilePath{"bar"}
	sub, err = parent.Extend(child)
	require.NoError(t, err)
	require.Equal(t, bass.FilePath{"foo/bar"}, sub)

	child = bass.CommandPath{"bar"}
	sub, err = parent.Extend(child)
	require.Nil(t, sub)
	require.Error(t, err)
}

func TestFilePathDecode(t *testing.T) {
	var foo bass.FilePath
	err := bass.FilePath{"foo"}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, bass.FilePath{"foo"}, foo)

	err = bass.FilePath{"bar"}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, bass.FilePath{"bar"}, foo)

	var path_ bass.Path
	err = bass.FilePath{"bar"}.Decode(&path_)
	require.NoError(t, err)
	require.Equal(t, bass.FilePath{"bar"}, path_)

	var comb bass.Combiner
	err = bass.FilePath{"foo"}.Decode(&comb)
	require.NoError(t, err)
	require.Equal(t, bass.FilePath{"foo"}, comb)

	var app bass.Applicative
	err = bass.FilePath{"foo"}.Decode(&app)
	require.NoError(t, err)
	require.Equal(t, bass.FilePath{"foo"}, comb)
}

func TestFilePathFromSlash(t *testing.T) {
	require.Equal(t,
		bass.FilePath{"hello/foo/bar"}.FromSlash(),
		filepath.FromSlash("./hello/foo/bar"),
	)

	require.Equal(t,
		bass.FilePath{"./hello/foo/bar"}.FromSlash(),
		filepath.FromSlash("./hello/foo/bar"),
	)

	require.Equal(t,
		bass.FilePath{"/hello/foo/bar"}.FromSlash(),
		filepath.FromSlash("/hello/foo/bar"),
	)
}

func TestFilePathEqual(t *testing.T) {
	require.True(t, bass.FilePath{"hello"}.Equal(bass.FilePath{"hello"}))
	require.True(t, bass.FilePath{""}.Equal(bass.FilePath{""}))
	require.False(t, bass.FilePath{"hello"}.Equal(bass.FilePath{""}))
	require.False(t, bass.FilePath{"hello"}.Equal(bass.DirPath{"hello"}))
	require.False(t, bass.FilePath{"hello"}.Equal(bass.CommandPath{"hello"}))
	require.True(t, bass.FilePath{"hello"}.Equal(wrappedValue{bass.FilePath{"hello"}}))
	require.False(t, bass.FilePath{"hello"}.Equal(wrappedValue{bass.FilePath{""}}))
}

func TestFilePathIsDir(t *testing.T) {
	require.False(t, bass.FilePath{"hello"}.IsDir())
}

func TestFilePathExtend(t *testing.T) {
	var parent, child bass.Path

	parent = bass.FilePath{"foo"}

	child = bass.DirPath{"bar"}
	_, err := parent.Extend(child)
	require.Error(t, err)

	child = bass.FilePath{"bar"}
	_, err = parent.Extend(child)
	require.Error(t, err)

	child = bass.CommandPath{"bar"}
	_, err = parent.Extend(child)
	require.Error(t, err)
}

func TestFilePathCall(t *testing.T) {
	scope := bass.NewEmptyScope()
	val := bass.FilePath{"foo"}

	scope.Set("foo", bass.String("hello"))

	res, err := Call(val, scope, bass.NewList(bass.Symbol("foo")))
	require.NoError(t, err)
	require.Equal(t, res, bass.Bindings{
		"path":     bass.FilePath{"foo"},
		"stdin":    bass.NewList(bass.String("hello")),
		"response": bass.Bindings{"stdout": bass.Bool(true)}.Scope(),
	}.Scope())
}

func TestFilePathUnwrap(t *testing.T) {
	scope := bass.NewEmptyScope()
	val := bass.FilePath{"echo"}

	res, err := Call(val.Unwrap(), scope, bass.NewList(bass.String("hello")))
	require.NoError(t, err)
	require.Equal(t, res, bass.Bindings{
		"path":     bass.FilePath{"echo"},
		"stdin":    bass.NewList(bass.String("hello")),
		"response": bass.Bindings{"stdout": bass.Bool(true)}.Scope(),
	}.Scope())
}

func TestCommandPathDecode(t *testing.T) {
	var foo bass.CommandPath
	err := bass.CommandPath{"foo"}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, bass.CommandPath{"foo"}, foo)

	err = bass.CommandPath{"bar"}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, bass.CommandPath{"bar"}, foo)

	var path_ bass.Path
	err = bass.CommandPath{"bar"}.Decode(&path_)
	require.NoError(t, err)
	require.Equal(t, bass.CommandPath{"bar"}, path_)

	var comb bass.Combiner
	err = bass.CommandPath{"foo"}.Decode(&comb)
	require.NoError(t, err)
	require.Equal(t, bass.CommandPath{"foo"}, comb)

	var app bass.Applicative
	err = bass.CommandPath{"foo"}.Decode(&app)
	require.NoError(t, err)
	require.Equal(t, bass.CommandPath{"foo"}, comb)
}

func TestCommandPathEqual(t *testing.T) {
	require.True(t, bass.CommandPath{"hello"}.Equal(bass.CommandPath{"hello"}))
	require.True(t, bass.CommandPath{""}.Equal(bass.CommandPath{""}))
	require.False(t, bass.CommandPath{"hello"}.Equal(bass.CommandPath{""}))
	require.False(t, bass.CommandPath{"hello"}.Equal(bass.DirPath{"hello"}))
	require.False(t, bass.CommandPath{"hello"}.Equal(bass.FilePath{"hello"}))
	require.True(t, bass.CommandPath{"hello"}.Equal(wrappedValue{bass.CommandPath{"hello"}}))
	require.False(t, bass.CommandPath{"hello"}.Equal(wrappedValue{bass.CommandPath{""}}))
}

func TestCommandPathExtend(t *testing.T) {
	var parent, child bass.Path

	parent = bass.CommandPath{"foo"}

	child = bass.DirPath{"bar"}
	_, err := parent.Extend(child)
	require.Error(t, err)

	child = bass.FilePath{"bar"}
	_, err = parent.Extend(child)
	require.Error(t, err)

	child = bass.CommandPath{"bar"}
	_, err = parent.Extend(child)
	require.Error(t, err)
}

func TestCommandPathCall(t *testing.T) {
	scope := bass.NewEmptyScope()
	val := bass.CommandPath{"echo"}

	scope.Set("foo", bass.String("hello"))

	res, err := Call(val, scope, bass.NewList(bass.Symbol("foo")))
	require.NoError(t, err)
	require.Equal(t, res, bass.Bindings{
		"path":     bass.CommandPath{"echo"},
		"stdin":    bass.NewList(bass.String("hello")),
		"response": bass.Bindings{"stdout": bass.Bool(true)}.Scope(),
	}.Scope())
}

func TestCommandPathUnwrap(t *testing.T) {
	scope := bass.NewEmptyScope()
	val := bass.CommandPath{"echo"}

	res, err := Call(val.Unwrap(), scope, bass.NewList(bass.String("hello")))
	require.NoError(t, err)
	require.Equal(t, res, bass.Bindings{
		"path":     bass.CommandPath{"echo"},
		"stdin":    bass.NewList(bass.String("hello")),
		"response": bass.Bindings{"stdout": bass.Bool(true)}.Scope(),
	}.Scope())
}

func TestExtendPathDecode(t *testing.T) {
	var foo bass.ExtendPath
	err := bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}, foo)

	err = bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"baz"},
	}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"baz"},
	}, foo)

	var path_ bass.Path
	err = bass.ExtendPath{Parent: bass.Symbol("foo"), Child: bass.FilePath{"bar"}}.Decode(&path_)
	require.Error(t, err)

	var comb bass.Combiner
	err = bass.ExtendPath{Parent: bass.Symbol("foo"), Child: bass.FilePath{"bar"}}.Decode(&comb)
	require.Error(t, err)
}

func TestExtendPathEqual(t *testing.T) {
	require.True(t, bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}.Equal(bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}))
	require.False(t, bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}.Equal(bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"baz"},
	}))
	require.True(t, bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}.Equal(wrappedValue{bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}}))
	require.False(t, bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"bar"},
	}.Equal(wrappedValue{bass.ExtendPath{
		Parent: bass.Symbol("foo"),
		Child:  bass.FilePath{"baz"},
	}}))
}
