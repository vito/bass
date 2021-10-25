package runtimes_test

import (
	"testing"

	bass "github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/is"
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
	is := is.New(t)

	workload := bass.Workload{
		Path: bass.RunPath{
			Cmd: &bass.CommandPath{Command: "run"},
		},
	}

	t.Run("command path", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(workload)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
		})
	})

	entrypointWl := workload
	entrypointWl.Entrypoint = []bass.Value{bass.CommandPath{Command: "bash"}}

	t.Run("command in entrypoint", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(entrypointWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Entrypoint: []string{"bash"},
			Args:       []string{"run"},
		})
	})

	fileWl := workload
	fileWl.Path = bass.RunPath{
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

	pathWl := workload
	pathWl.Path = bass.RunPath{
		WorkloadFile: &wlFile,
	}

	t.Run("mounts workload run path", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(pathWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"./" + wlName + "/some-file"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlFile),
					Target: "./" + wlName + "/some-file",
				},
			},
		})
	})

	argsWl := workload
	argsWl.Args = []bass.Value{wlFile, bass.DirPath{Path: "data"}}

	t.Run("paths in args", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(argsWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run", "./" + wlName + "/some-file", "./data/"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlFile),
					Target: "./" + wlName + "/some-file",
				},
			},
		})
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
		is := is.New(t)
		cmd, err := runtimes.NewCommand(stdinWl)
		is.NoErr(err)
		is.True(cmd.Stdin[0].Equal(
			bass.Bindings{
				"context": bass.String("./" + wlName + "/some-file"),
				"out":     bass.String("./data/"),
			}.Scope()))
		is.True(cmd.Stdin[1].Equal(bass.Int(42)))
		is.Equal(cmd.Mounts, []runtimes.CommandMount{
			{
				Source: bass.WorkloadPathSource(wlFile),
				Target: "./" + wlName + "/some-file",
			},
		})
	})

	envWlp := wlFile
	envWlp.Path = bass.FileOrDirPath{
		File: &bass.FilePath{Path: "env-file"},
	}

	envWlpWl := workload
	envWlpWl.Env = bass.Bindings{
		"INPUT": envWlp}.Scope()

	t.Run("workload paths in env", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(envWlpWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Env:  []string{"INPUT=./" + wlName + "/env-file"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(envWlp),
					Target: "./" + wlName + "/env-file",
				},
			},
		})
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
		is := is.New(t)
		cmd, err := runtimes.NewCommand(envArgWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Env:  []string{"FOO=foo=./some/dir/!"},
		})
	})

	dirWlpWl := workload
	dirWlpWl.Dir = &bass.RunDirPath{
		WorkloadDir: &wlDir,
	}

	t.Run("workload path as dir", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(dirWlpWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Dir:  strptr("./" + wlName + "/some-dir/"),
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlDir),
					Target: "./" + wlName + "/some-dir/",
				},
			},
		})
	})

	dupeWl := workload
	dupeWl.Path = bass.RunPath{
		WorkloadFile: &wlFile,
	}
	dupeWl.Args = []bass.Value{wlDir}
	dupeWl.Stdin = []bass.Value{wlFile}
	dupeWl.Env = bass.Bindings{"INPUT": wlFile}.Scope()
	dupeWl.Dir = &bass.RunDirPath{
		WorkloadDir: &wlDir,
	}

	t.Run("does not mount same path twice", func(t *testing.T) {
		is := is.New(t)
		cmd, err := runtimes.NewCommand(dupeWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args:  []string{"../../" + wlName + "/some-file", "../../" + wlName + "/some-dir/"},
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
		})
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
		is := is.New(t)
		cmd, err := runtimes.NewCommand(mountsWl)
		is.NoErr(err)
		is.Equal(cmd, runtimes.Command{
			Args: []string{"run"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlFile),
					Target: "./dir/",
				},
			},
		})
	})
}

func TestNewCommandInDir(t *testing.T) {
	is := is.New(t)

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
	is.NoErr(err)
	is.Equal(

		cmd, runtimes.Command{
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
		})

}

func strptr(s string) *string {
	return &s
}
