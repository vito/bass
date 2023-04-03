package runtimes_test

import (
	"context"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/is"
)

var scratch = bass.Thunk{}

var thunk = bass.Thunk{
	Args: []bass.Value{
		bass.FilePath{Path: "yo"},
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

var svcThunk = bass.Thunk{
	Args: []bass.Value{
		bass.FilePath{Path: "yo"},
	},
	Ports: []bass.ThunkPort{
		{
			Name: "http",
			Port: 80,
		},
		{
			Name: "https",
			Port: 443,
		},
	},
	TLS: &bass.ThunkTLS{
		Cert: bass.FilePath{Path: "cert"},
		Key:  bass.FilePath{Path: "key"},
	},
}

var thunkAddr = bass.ThunkAddr{
	Thunk:  svcThunk,
	Port:   "http",
	Format: "some://$host:$port/addr",
}

func init() {
	var err error
	thunkName, err = thunk.Hash()
	if err != nil {
		panic(err)
	}
}

type FakeStarter struct {
	Started     bass.Thunk
	StartResult runtimes.StartResult
}

// Start starts the thunk and waits for its ports to be ready.
func (starter *FakeStarter) Start(ctx context.Context, thunk bass.Thunk) (runtimes.StartResult, error) {
	starter.Started = thunk
	return starter.StartResult, nil
}

func TestNewCommand(t *testing.T) {
	is := is.New(t)

	thunk := bass.Thunk{
		Args: []bass.Value{
			bass.CommandPath{Command: "run"},
		},
	}

	ctx := context.Background()
	ctx, runs := bass.TrackRuns(ctx)
	defer runs.Wait()

	starter := &FakeStarter{}

	t.Run("command path", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(ctx, starter, thunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
		})
	})

	t.Run("file run path", func(t *testing.T) {
		fileCmdThunk := thunk
		fileCmdThunk.Args = []bass.Value{bass.FilePath{Path: "run"}}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(ctx, starter, fileCmdThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"./run"},
		})
	})

	t.Run("using a thunk file as a command", func(t *testing.T) {
		thunkFileCmdThunk := thunk
		thunkFileCmdThunk.Args = []bass.Value{thunkFile}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(ctx, starter, thunkFileCmdThunk)
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
		thunk.Args = []bass.Value{cache}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(ctx, starter, thunk)
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
		argsThunk.Args = append(argsThunk.Args, thunkFile, bass.DirPath{Path: "data"})

		is := is.New(t)
		cmd, err := runtimes.NewCommand(ctx, starter, argsThunk)
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
		cmd, err := runtimes.NewCommand(ctx, starter, stdinThunk)
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

	t.Run("paths in env", func(t *testing.T) {
		envThunkPath := thunkFile
		envThunkPath.Path = bass.FileOrDirPath{
			File: &bass.FilePath{Path: "env-file"},
		}

		envThunkPathThunk := thunk
		envThunkPathThunk.Env = bass.Bindings{
			"INPUT": envThunkPath,
		}.Scope()

		is := is.New(t)
		cmd, err := runtimes.NewCommand(ctx, starter, envThunkPathThunk)
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

	t.Run("nulls in env", func(t *testing.T) {
		envTombstoneThunk := thunk.WithEnv(
			bass.Bindings{
				"FOO": bass.String("hello"),
				"BAR": bass.String("world"),
			}.Scope(),
		).WithEnv(
			bass.Bindings{
				"FOO": bass.Null{},
			}.Scope(),
		)

		is := is.New(t)
		cmd, err := runtimes.NewCommand(ctx, starter, envTombstoneThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Env:  []string{"BAR=world"},
		})
	})

	t.Run("concatenating", func(t *testing.T) {
		concatThunk := thunk
		concatThunk.Args = []bass.Value{
			bass.CommandPath{Command: "run"},
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
		cmd, err := runtimes.NewCommand(ctx, starter, concatThunk)
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
		cmd, err := runtimes.NewCommand(ctx, starter, dirThunkPathThunk)
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
		dupeMountThunk.Args = []bass.Value{thunkFile, thunkDir}
		dupeMountThunk.Stdin = []bass.Value{thunkFile}
		dupeMountThunk.Env = bass.Bindings{"INPUT": thunkFile}.Scope()
		dupeMountThunk.Dir = &bass.ThunkDir{
			ThunkDir: &thunkDir,
		}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(ctx, starter, dupeMountThunk)
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
		cmd, err := runtimes.NewCommand(ctx, starter, mountsThunk)
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

	t.Run("addrs in args", func(t *testing.T) {
		argsThunk := thunk
		argsThunk.Args = []bass.Value{
			bass.CommandPath{Command: "run"},
			thunkAddr,
		}

		starter := &FakeStarter{
			StartResult: runtimes.StartResult{
				Ports: runtimes.PortInfos{
					"http": bass.Bindings{
						"host": bass.String("drew"),
						"port": bass.Int(6455),
					}.Scope(),
				},
			},
		}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(ctx, starter, argsThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args:     []string{"run", "some://drew:6455/addr"},
			Services: []bass.Thunk{svcThunk},
		})

		is.Equal(starter.Started, svcThunk)
	})

	t.Run("addrs in stdin", func(t *testing.T) {
		stdinThunk := thunk
		stdinThunk.Stdin = []bass.Value{
			bass.Bindings{
				"addr": thunkAddr,
			}.Scope(),
			bass.Int(42),
		}

		starter := &FakeStarter{
			StartResult: runtimes.StartResult{
				Ports: runtimes.PortInfos{
					"http": bass.Bindings{
						"host": bass.String("drew"),
						"port": bass.Int(6455),
					}.Scope(),
				},
			},
		}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(ctx, starter, stdinThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args:     []string{"run"},
			Stdin:    []byte(`{"addr":"some://drew:6455/addr"}` + "\n42\n"),
			Services: []bass.Thunk{svcThunk},
		})

		is.Equal(starter.Started, svcThunk)
	})

	t.Run("addrs in env", func(t *testing.T) {
		envThunkPathThunk := thunk
		envThunkPathThunk.Env = bass.Bindings{
			"ADDR": thunkAddr,
		}.Scope()

		starter := &FakeStarter{
			StartResult: runtimes.StartResult{
				Ports: runtimes.PortInfos{
					"http": bass.Bindings{
						"host": bass.String("drew"),
						"port": bass.Int(6455),
					}.Scope(),
				},
			},
		}

		is := is.New(t)
		cmd, err := runtimes.NewCommand(ctx, starter, envThunkPathThunk)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args:     []string{"run"},
			Env:      []string{"ADDR=some://drew:6455/addr"},
			Services: []bass.Thunk{svcThunk},
		})

		is.Equal(starter.Started, svcThunk)
	})
}

func TestNewCommandInDir(t *testing.T) {
	is := is.New(t)

	thunk := bass.Thunk{
		Args: []bass.Value{
			bass.CommandPath{Command: "run"},
		},
		Dir: &bass.ThunkDir{
			ThunkDir: &thunkDir,
		},
		Stdin: []bass.Value{
			thunkFile,
		},
	}

	ctx := context.Background()
	ctx, runs := bass.TrackRuns(ctx)
	defer runs.Wait()

	starter := &FakeStarter{}

	cmd, err := runtimes.NewCommand(ctx, starter, thunk)
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
