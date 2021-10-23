package runtimes_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	bass "github.com/vito/bass"
	"github.com/vito/bass/runtimes"
)

var wl = bass.Workload{
	Path: bass.RunPath{
		File: &bass.FilePath{Path: "yo"},
	},
}

var wlFile = bass.WorkloadPath{
	Workload: wl,
	Path: bass.FileOrDirPath{
		File: &bass.FilePath{Path: "some-file"},
	},
}

var wlDir = bass.WorkloadPath{
	Workload: wl,
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
	workload := bass.Workload{
		Path: bass.RunPath{
			Cmd: &bass.CommandPath{Command: "run"},
		},
	}

	t.Run("command path", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(workload)
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
		}, cmd)
	})

	entrypointWl := workload
	entrypointWl.Entrypoint = []bass.Value{bass.CommandPath{Command: "bash"}}

	t.Run("command in entrypoint", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(entrypointWl)
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Entrypoint: []string{"bash"},
			Args:       []string{"run"},
		}, cmd)
	})

	fileWl := workload
	fileWl.Path = bass.RunPath{
		File: &bass.FilePath{Path: "run"},
	}

	t.Run("file run path", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(fileWl)
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"./run"},
		}, cmd)
	})

	pathWl := workload
	pathWl.Path = bass.RunPath{
		WorkloadFile: &wlFile,
	}

	t.Run("mounts workload run path", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(pathWl)
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"./" + wlName + "/some-file"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlFile),
					Target: "./" + wlName + "/some-file",
				},
			},
		}, cmd)
	})

	argsWl := workload
	argsWl.Args = []bass.Value{wlFile, bass.DirPath{Path: "data"}}

	t.Run("paths in args", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(argsWl)
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run", "./" + wlName + "/some-file", "./data/"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlFile),
					Target: "./" + wlName + "/some-file",
				},
			},
		}, cmd)
	})

	stdinWl := workload
	stdinWl.Stdin = []bass.Value{
		bass.Bindings{
			"context": wlFile,
			"out":     bass.DirPath{Path: "data"},
		}.Scope(),
		bass.Int(42),
	}

	t.Run("paths in stdin", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(stdinWl)
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
			Stdin: []bass.Value{
				bass.Bindings{
					"context": bass.String("./" + wlName + "/some-file"),
					"out":     bass.String("./data/"),
				}.Scope(),
				bass.Int(42),
			},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlFile),
					Target: "./" + wlName + "/some-file",
				},
			},
		}, cmd)
	})

	envWlp := wlFile
	envWlp.Path = bass.FileOrDirPath{
		File: &bass.FilePath{Path: "env-file"},
	}

	envWlpWl := workload
	envWlpWl.Env = bass.Bindings{
		"INPUT": envWlp}.Scope()

	t.Run("workload paths in env", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(envWlpWl)
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
			Env:  []string{"INPUT=./" + wlName + "/env-file"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(envWlp),
					Target: "./" + wlName + "/env-file",
				},
			},
		}, cmd)
	})

	envArgWl := workload
	envArgWl.Env = bass.Bindings{
		"FOO": bass.Bindings{
			"arg": bass.NewList(
				bass.String("foo="),
				bass.DirPath{Path: "some/dir"},
				bass.String("!"),
			)}.Scope()}.Scope()

	t.Run("concatenating args", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(envArgWl)
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
			Env:  []string{"FOO=foo=./some/dir/!"},
		}, cmd)
	})

	dirWlpWl := workload
	dirWlpWl.Dir = &bass.RunDirPath{
		WorkloadDir: &wlDir,
	}

	t.Run("workload path as dir", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(dirWlpWl)
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
			Dir:  strptr("./" + wlName + "/some-dir/"),
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlDir),
					Target: "./" + wlName + "/some-dir/",
				},
			},
		}, cmd)
	})

	dupeWl := workload
	dupeWl.Path = bass.RunPath{
		WorkloadFile: &wlFile,
	}
	dupeWl.Args = []bass.Value{wlFile}
	dupeWl.Stdin = []bass.Value{wlFile}
	dupeWl.Env = bass.Bindings{"INPUT": wlFile}.Scope()
	dupeWl.Dir = &bass.RunDirPath{
		WorkloadDir: &wlDir,
	}

	t.Run("does not mount same path twice", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(dupeWl)
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args:  []string{"../../" + wlName + "/some-file", "../../" + wlName + "/some-file"},
			Stdin: []bass.Value{bass.String("../../" + wlName + "/some-file")},
			Env:   []string{"INPUT=../../" + wlName + "/some-file"},
			Dir:   strptr("./" + wlName + "/some-dir/"),
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlDir),
					Target: "./" + wlName + "/some-dir/",
				},
				{
					Source: bass.WorkloadPathSource(wlFile),
					Target: "./" + wlName + "/some-file",
				},
			},
		}, cmd)
	})

	mountsWl := workload
	mountsWl.Mounts = []bass.RunMount{
		{
			Source: bass.WorkloadPathSource(wlFile),
			Target: bass.FileOrDirPath{
				Dir: &bass.DirPath{Path: "dir"},
			},
		},
	}

	t.Run("mounts", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(mountsWl)
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlFile),
					Target: "./dir/",
				},
			},
		}, cmd)
	})
}

func TestNewCommandInDir(t *testing.T) {
	workload := bass.Workload{
		Path: bass.RunPath{
			Cmd: &bass.CommandPath{Command: "run"},
		},
		Dir: &bass.RunDirPath{
			WorkloadDir: &wlDir,
		},
		Stdin: []bass.Value{
			wlFile,
		},
	}

	cmd, err := runtimes.NewCommand(workload)
	require.NoError(t, err)
	require.Equal(t, runtimes.Command{
		Args: []string{"run"},
		Dir:  strptr("./" + wlName + "/some-dir/"),
		Stdin: []bass.Value{
			bass.String("../../" + wlName + "/some-file"),
		},
		Mounts: []runtimes.CommandMount{
			{
				Source: bass.WorkloadPathSource(wlDir),
				Target: "./" + wlName + "/some-dir/",
			},
			{
				Source: bass.WorkloadPathSource(wlFile),
				Target: "./" + wlName + "/some-file",
			},
		},
	}, cmd)
}

func strptr(s string) *string {
	return &s
}
