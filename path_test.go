package bass_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestDirectoryPathDecode(t *testing.T) {
	var foo bass.DirectoryPath
	err := bass.DirectoryPath{"foo"}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, bass.DirectoryPath{"foo"}, foo)

	err = bass.DirectoryPath{"bar"}.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, bass.DirectoryPath{"bar"}, foo)

	var path_ bass.Path
	err = bass.DirectoryPath{"bar"}.Decode(&path_)
	require.NoError(t, err)
	require.Equal(t, bass.DirectoryPath{"bar"}, path_)

	var comb bass.Combiner
	err = bass.DirectoryPath{"foo"}.Decode(&comb)
	require.Error(t, err)

	var app bass.Applicative
	err = bass.DirectoryPath{"foo"}.Decode(&app)
	require.Error(t, err)
}

func TestDirectoryPathEqual(t *testing.T) {
	require.True(t, bass.DirectoryPath{"hello"}.Equal(bass.DirectoryPath{"hello"}))
	require.True(t, bass.DirectoryPath{""}.Equal(bass.DirectoryPath{""}))
	require.False(t, bass.DirectoryPath{"hello"}.Equal(bass.DirectoryPath{""}))
	require.False(t, bass.DirectoryPath{"hello"}.Equal(bass.FilePath{"hello"}))
	require.False(t, bass.DirectoryPath{"hello"}.Equal(bass.CommandPath{"hello"}))
	require.True(t, bass.DirectoryPath{"hello"}.Equal(wrappedValue{bass.DirectoryPath{"hello"}}))
	require.False(t, bass.DirectoryPath{"hello"}.Equal(wrappedValue{bass.DirectoryPath{""}}))
}

func TestDirectoryPathResolve(t *testing.T) {
	// need an absolute path in a platform-neutral way
	cwd, err := os.Getwd()
	require.NoError(t, err)

	path_, err := bass.DirectoryPath{"bar"}.Resolve(cwd)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cwd, "bar"), path_)

	path_, err = bass.DirectoryPath{cwd}.Resolve(cwd)
	require.NoError(t, err)
	require.Equal(t, cwd, path_)
}

func TestDirectoryPathExtend(t *testing.T) {
	var parent, child bass.Path

	parent = bass.DirectoryPath{"foo"}

	child = bass.DirectoryPath{"bar"}
	sub, err := parent.Extend(child)
	require.NoError(t, err)
	require.Equal(t, bass.DirectoryPath{"foo/bar"}, sub)

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

func TestFilePathEqual(t *testing.T) {
	require.True(t, bass.FilePath{"hello"}.Equal(bass.FilePath{"hello"}))
	require.True(t, bass.FilePath{""}.Equal(bass.FilePath{""}))
	require.False(t, bass.FilePath{"hello"}.Equal(bass.FilePath{""}))
	require.False(t, bass.FilePath{"hello"}.Equal(bass.DirectoryPath{"hello"}))
	require.False(t, bass.FilePath{"hello"}.Equal(bass.CommandPath{"hello"}))
	require.True(t, bass.FilePath{"hello"}.Equal(wrappedValue{bass.FilePath{"hello"}}))
	require.False(t, bass.FilePath{"hello"}.Equal(wrappedValue{bass.FilePath{""}}))
}

func TestFilePathResolve(t *testing.T) {
	// need an absolute path in a platform-neutral way
	cwd, err := os.Getwd()
	require.NoError(t, err)

	path_, err := bass.FilePath{"bar"}.Resolve(cwd)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cwd, "bar"), path_)

	path_, err = bass.FilePath{cwd}.Resolve(cwd)
	require.NoError(t, err)
	require.Equal(t, cwd, path_)
}

func TestFilePathExtend(t *testing.T) {
	var parent, child bass.Path

	parent = bass.FilePath{"foo"}

	child = bass.DirectoryPath{"bar"}
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
	env := bass.NewEnv()
	val := bass.FilePath{"foo"}

	env.Set("foo", bass.String("hello"))

	res, err := Call(val, env, bass.NewList(bass.Symbol("foo")))
	require.NoError(t, err)
	require.Equal(t, res, bass.Object{
		"platform": bass.Object{
			"native": bass.Bool(true),
		},
		"command": bass.Object{
			"path":  bass.FilePath{"foo"},
			"stdin": bass.NewList(bass.String("hello")),
		},
	})
}

func TestFilePathUnwrap(t *testing.T) {
	env := bass.NewEnv()
	val := bass.FilePath{"echo"}

	res, err := Call(val.Unwrap(), env, bass.NewList(bass.String("hello")))
	require.NoError(t, err)
	require.Equal(t, res, bass.Object{
		"platform": bass.Object{
			"native": bass.Bool(true),
		},
		"command": bass.Object{
			"path":  bass.FilePath{"echo"},
			"stdin": bass.NewList(bass.String("hello")),
		},
	})
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
	require.False(t, bass.CommandPath{"hello"}.Equal(bass.DirectoryPath{"hello"}))
	require.False(t, bass.CommandPath{"hello"}.Equal(bass.FilePath{"hello"}))
	require.True(t, bass.CommandPath{"hello"}.Equal(wrappedValue{bass.CommandPath{"hello"}}))
	require.False(t, bass.CommandPath{"hello"}.Equal(wrappedValue{bass.CommandPath{""}}))
}

func TestCommandPathResolve(t *testing.T) {
	path_, err := bass.CommandPath{"go"}.Resolve("./foo")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(runtime.GOROOT(), "bin", "go"), path_)
}

func TestCommandPathExtend(t *testing.T) {
	var parent, child bass.Path

	parent = bass.CommandPath{"foo"}

	child = bass.DirectoryPath{"bar"}
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
	env := bass.NewEnv()
	val := bass.CommandPath{"echo"}

	env.Set("foo", bass.String("hello"))

	res, err := Call(val, env, bass.NewList(bass.Symbol("foo")))
	require.NoError(t, err)
	require.Equal(t, res, bass.Object{
		"platform": bass.Object{
			"native": bass.Bool(true),
		},
		"command": bass.Object{
			"path":  bass.CommandPath{"echo"},
			"stdin": bass.NewList(bass.String("hello")),
		},
	})
}

func TestCommandPathUnwrap(t *testing.T) {
	env := bass.NewEnv()
	val := bass.CommandPath{"echo"}

	res, err := Call(val.Unwrap(), env, bass.NewList(bass.String("hello")))
	require.NoError(t, err)
	require.Equal(t, res, bass.Object{
		"platform": bass.Object{
			"native": bass.Bool(true),
		},
		"command": bass.Object{
			"path":  bass.CommandPath{"echo"},
			"stdin": bass.NewList(bass.String("hello")),
		},
	})
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
