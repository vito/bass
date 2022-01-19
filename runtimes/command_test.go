package runtimes_test

import (
	"testing"

	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/is"
)

var wl = bass.Thunk{
	Cmd: bass.ThunkCmd{
		File: &bass.FilePath{Path: "yo"},
	},
}

var wlFile = bass.ThunkPath{
	Thunk: wl,
	Path: bass.FileOrDirPath{
		File: &bass.FilePath{Path: "some-file"},
	},
}

var wlDir = bass.ThunkPath{
	Thunk: wl,
	Path: bass.FileOrDirPath{
		Dir: &bass.DirPath{Path: "some-dir"},
	},
}

// NB: must be updated whenever the hashing changes
var wlName string

func init() {
	var err error
	wlName, err = wl.SHA1()
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

	fileWl := thunk
	fileWl.Cmd = bass.ThunkCmd{
		File: &bass.FilePath{Path: "run"},
	}

	t.Run("file run path", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(fileWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"./run"},
		})
	})

	pathWl := thunk
	pathWl.Cmd = bass.ThunkCmd{
		ThunkFile: &wlFile,
	}

	t.Run("mounts thunk run path", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(pathWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"./" + wlName + "/some-file"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &wlFile,
					},
					Target: "./" + wlName + "/some-file",
				},
			},
		})
	})

	argsWl := thunk
	argsWl.Args = []bass.Value{wlFile, bass.DirPath{Path: "data"}}

	t.Run("paths in args", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(argsWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run", "./" + wlName + "/some-file", "./data/"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &wlFile,
					},
					Target: "./" + wlName + "/some-file",
				},
			},
		})
	})

	stdinWl := thunk
	stdinWl.Stdin = []bass.Value{
		bass.Bindings{
			"context": wlFile,
			"out":     bass.DirPath{Path: "data"},
		}.Scope(),
		bass.Int(42),
	}

	t.Run("paths in stdin", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(stdinWl)
		is.NoErr(err)
		is.Equal(cmd.Stdin, []byte(`{"context":"./`+wlName+`/some-file","out":"./data/"}`+"\n42\n"))
		is.Equal(cmd.Mounts, []runtimes.CommandMount{
			{
				Source: bass.ThunkMountSource{
					ThunkPath: &wlFile,
				},
				Target: "./" + wlName + "/some-file",
			},
		})
	})

	envWlp := wlFile
	envWlp.Path = bass.FileOrDirPath{
		File: &bass.FilePath{Path: "env-file"},
	}

	envWlpWl := thunk
	envWlpWl.Env = bass.Bindings{
		"INPUT": envWlp}.Scope()

	t.Run("thunk paths in env", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(envWlpWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Env:  []string{"INPUT=./" + wlName + "/env-file"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &envWlp,
					},
					Target: "./" + wlName + "/env-file",
				},
			},
		})
	})

	envArgWl := thunk
	envArgWl.Env = bass.Bindings{
		"FOO": bass.Bindings{
			"arg": bass.NewList(
				bass.String("foo="),
				bass.DirPath{Path: "some/dir"},
				bass.String("!"),
			)}.Scope()}.Scope()

	t.Run("concatenating args", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(envArgWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Env:  []string{"FOO=foo=./some/dir/!"},
		})
	})

	dirWlpWl := thunk
	dirWlpWl.Dir = &bass.ThunkDir{
		ThunkDir: &wlDir,
	}

	t.Run("thunk path as dir", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(dirWlpWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Dir:  strptr("./" + wlName + "/some-dir/"),
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &wlDir,
					},
					Target: "./" + wlName + "/some-dir/",
				},
			},
		})
	})

	dupeWl := thunk
	dupeWl.Cmd = bass.ThunkCmd{
		ThunkFile: &wlFile,
	}
	dupeWl.Args = []bass.Value{wlDir}
	dupeWl.Stdin = []bass.Value{wlFile}
	dupeWl.Env = bass.Bindings{"INPUT": wlFile}.Scope()
	dupeWl.Dir = &bass.ThunkDir{
		ThunkDir: &wlDir,
	}

	t.Run("does not mount same path twice", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(dupeWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args:  []string{"../../" + wlName + "/some-file", "../../" + wlName + "/some-dir/"},
			Stdin: []byte("\"../../" + wlName + "/some-file\"\n"),
			Env:   []string{"INPUT=../../" + wlName + "/some-file"},
			Dir:   strptr("./" + wlName + "/some-dir/"),
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &wlDir,
					},
					Target: "./" + wlName + "/some-dir/",
				},
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &wlFile,
					},
					Target: "./" + wlName + "/some-file",
				},
			},
		})
	})

	mountsWl := thunk
	mountsWl.Mounts = []bass.ThunkMount{
		{
			Source: bass.ThunkMountSource{
				ThunkPath: &wlFile,
			},
			Target: bass.FileOrDirPath{
				Dir: &bass.DirPath{Path: "dir"},
			},
		},
	}

	t.Run("mounts", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(mountsWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.ThunkMountSource{
						ThunkPath: &wlFile,
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
			ThunkDir: &wlDir,
		},
		Stdin: []bass.Value{
			wlFile,
		},
	}

	cmd, err := runtimes.NewCommand(thunk)
	is.NoErr(err)
	is.Equal(cmd, runtimes.Command{
		Args:  []string{"run"},
		Dir:   strptr("./" + wlName + "/some-dir/"),
		Stdin: []byte("\"../../" + wlName + "/some-file\"\n"),
		Mounts: []runtimes.CommandMount{
			{
				Source: bass.ThunkMountSource{
					ThunkPath: &wlDir,
				},
				Target: "./" + wlName + "/some-dir/",
			},
			{
				Source: bass.ThunkMountSource{
					ThunkPath: &wlFile,
				},
				Target: "./" + wlName + "/some-file",
			},
		},
	})
}

func strptr(s string) *string {
	return &s
}
