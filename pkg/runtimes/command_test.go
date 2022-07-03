package runtimes_test

import (
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/is"
)

var thunk = bass.Thunk{
	Cmd: bass.ThunkCmd{
		File: &bass.FilePath{Path: "yo"},
	},
}

var thunkFile = bass.ThunkPath{
	Thunk: thunk,
	Path: bass.FileOrDirPath{
		File: &bass.FilePath{Path: "some-file"},
	},
}

var thunkDir = bass.ThunkPath{
	Thunk: thunk,
	Path: bass.FileOrDirPath{
		Dir: &bass.DirPath{Path: "some-dir"},
	},
}

// NB: must be updated whenever the hashing changes
var thunkName string

func init() {
	var err error
	thunkName, err = thunk.SHA256()
	if err != nil {
		panic(err)
	}
}

func TestNewCommand(t *testing.T) {
	is := is.New(t)

	thunk := bass.Thunk{
		Cmd: bass.ThunkCmd{
			Cmd: &bass.CommandPath{Command: "run"},
		},
	}

	t.Run("command path", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(thunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
		})
	})

	t.Run("file run path", func(t *testing.T) {
		fileCmdThunk := thunk
		fileCmdThunk.Cmd = bass.ThunkCmd{
			File: &bass.FilePath{Path: "run"},
		}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(fileCmdThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"./run"},
		})
	})

	t.Run("using a thunk file as a command", func(t *testing.T) {
		thunkFileCmdThunk := thunk
		thunkFileCmdThunk.Cmd = bass.ThunkCmd{
			Thunk: &thunkFile,
		}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(thunkFileCmdThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"./" + thunkName + "/some-file"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &thunkFile,
					},
					Target: "./" + thunkName + "/some-file",
				},
			},
		})
	})

	t.Run("using a cache file as a command", func(t *testing.T) {
		cache := bass.NewCachePath("some-cache", bass.ParseFileOrDirPath("./exe"))
		hash := cache.Hash()

		thunk := thunk
		thunk.Cmd = bass.ThunkCmd{
			Cache: &cache,
		}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(thunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"./" + hash + "/exe"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						Cache: &cache,
					},
					Target: "./" + hash + "/exe",
				},
			},
		})
	})

	t.Run("paths in args", func(t *testing.T) {
		argsThunk := thunk
		argsThunk.Args = []bass.Value{thunkFile, bass.DirPath{Path: "data"}}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(argsThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run", "./" + thunkName + "/some-file", "./data/"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &thunkFile,
					},
					Target: "./" + thunkName + "/some-file",
				},
			},
		})
	})

	t.Run("paths in stdin", func(t *testing.T) {
		stdinThunk := thunk
		stdinThunk.Stdin = []bass.Value{
			bass.Bindings{
				"context": thunkFile,
				"out":     bass.DirPath{Path: "data"},
			}.Scope(),
			bass.Int(42),
		}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(stdinThunk)
		is.NoErr(err)
		is.Equal(string(cmd.Stdin), `{"context":"./`+thunkName+`/some-file","out":"./data/"}`+"\n42\n")
		is.Equal(cmd.Mounts, []runtimes.CommandMount{
			{
				Source: bass.ThunkMountSource{
					ThunkPath: &thunkFile,
				},
				Target: "./" + thunkName + "/some-file",
			},
		})
	})

	t.Run("thunk paths in env", func(t *testing.T) {
		envThunkPath := thunkFile
		envThunkPath.Path = bass.FileOrDirPath{
			File: &bass.FilePath{Path: "env-file"},
		}

		envThunkPathThunk := thunk
		envThunkPathThunk.Env = bass.Bindings{
			"INPUT": envThunkPath,
		}.Scope()

		is := is.New(t)
		cmd, err := runtimes.NewCommand(envThunkPathThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Env:  []string{"INPUT=./" + thunkName + "/env-file"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &envThunkPath,
					},
					Target: "./" + thunkName + "/env-file",
				},
			},
		})
	})

	t.Run("concatenating", func(t *testing.T) {
		concatThunk := thunk
		concatThunk.Args = []bass.Value{
			bass.NewList(
				bass.String("--dir="),
				bass.DirPath{Path: "some/dir"},
				bass.String("!"),
			),
		}
		concatThunk.Env = bass.Bindings{
			"FOO": bass.NewList(
				bass.String("foo="),
				bass.DirPath{Path: "some/dir"},
				bass.String("!"),
			),
		}.Scope()

		is := is.New(t)
		cmd, err := runtimes.NewCommand(concatThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run", "--dir=./some/dir/!"},
			Env:  []string{"FOO=foo=./some/dir/!"},
		})
	})

	t.Run("thunk path as dir", func(t *testing.T) {
		dirThunkPathThunk := thunk
		dirThunkPathThunk.Dir = &bass.ThunkDir{
			ThunkDir: &thunkDir,
		}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(dirThunkPathThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Dir:  strptr("./" + thunkName + "/some-dir/"),
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &thunkDir,
					},
					Target: "./" + thunkName + "/some-dir/",
				},
			},
		})
	})

	t.Run("does not mount same path twice", func(t *testing.T) {
		dupeMountThunk := thunk
		dupeMountThunk.Cmd = bass.ThunkCmd{
			Thunk: &thunkFile,
		}
		dupeMountThunk.Args = []bass.Value{thunkDir}
		dupeMountThunk.Stdin = []bass.Value{thunkFile}
		dupeMountThunk.Env = bass.Bindings{"INPUT": thunkFile}.Scope()
		dupeMountThunk.Dir = &bass.ThunkDir{
			ThunkDir: &thunkDir,
		}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(dupeMountThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args:  []string{"../../" + thunkName + "/some-file", "../../" + thunkName + "/some-dir/"},
			Stdin: []byte("\"../../" + thunkName + "/some-file\"\n"),
			Env:   []string{"INPUT=../../" + thunkName + "/some-file"},
			Dir:   strptr("./" + thunkName + "/some-dir/"),
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &thunkDir,
					},
					Target: "./" + thunkName + "/some-dir/",
				},
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &thunkFile,
					},
					Target: "./" + thunkName + "/some-file",
				},
			},
		})
	})

	t.Run("mounts", func(t *testing.T) {
		mountsThunk := thunk
		mountsThunk.Mounts = []bass.ThunkMount{
			{
				Source: bass.ThunkMountSource{
					ThunkPath: &thunkFile,
				},
				Target: bass.FileOrDirPath{
					Dir: &bass.DirPath{Path: "dir"},
				},
			},
		}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(mountsThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &thunkFile,
					},
					Target: "./dir/",
				},
			},
		})
	})
}

func TestNewCommandInDir(t *testing.T) {
	is := is.New(t)

	thunk := bass.Thunk{
		Cmd: bass.ThunkCmd{
			Cmd: &bass.CommandPath{Command: "run"},
		},
		Dir: &bass.ThunkDir{
			ThunkDir: &thunkDir,
		},
		Stdin: []bass.Value{
			thunkFile,
		},
	}

	cmd, err := runtimes.NewCommand(thunk)
	is.NoErr(err)
	is.Equal(cmd, runtimes.Command{
		Args:  []string{"run"},
		Dir:   strptr("./" + thunkName + "/some-dir/"),
		Stdin: []byte("\"../../" + thunkName + "/some-file\"\n"),
		Mounts: []runtimes.CommandMount{
			{
				Source: bass.ThunkMountSource{
					ThunkPath: &thunkDir,
				},
				Target: "./" + thunkName + "/some-dir/",
			},
			{
				Source: bass.ThunkMountSource{
					ThunkPath: &thunkFile,
				},
				Target: "./" + thunkName + "/some-file",
			},
		},
	})
}

func strptr(s string) *string {
	return &s
}
