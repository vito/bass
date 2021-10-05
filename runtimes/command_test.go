package runtimes_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	bass "github.com/vito/bass"
	"github.com/vito/bass/runtimes"
)

func TestNewCommand(t *testing.T) {
	wlp := bass.WorkloadPath{
		Workload: bass.Workload{
			Path: bass.RunPath{
				File: &bass.FilePath{"yo"},
			},
		},
		Path: bass.FileOrDirPath{
			File: &bass.FilePath{"script"},
		},
	}

	workload := bass.Workload{
		Path: bass.RunPath{
			Cmd: &bass.CommandPath{"run"},
		},
	}

	t.Run("command path", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(workload, "/work-dir")
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
		}, cmd)
	})

	entrypointWl := workload
	entrypointWl.Entrypoint = []bass.Value{bass.CommandPath{"bash"}}

	t.Run("command in entrypoint", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(entrypointWl, "/work-dir")
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Entrypoint: []string{"bash"},
			Args:       []string{"run"},
		}, cmd)
	})

	fileWl := workload
	fileWl.Path = bass.RunPath{
		File: &bass.FilePath{"run"},
	}

	t.Run("file run path", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(fileWl, "/work-dir")
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"./run"},
		}, cmd)
	})

	pathWl := workload
	pathWl.Path = bass.RunPath{
		WorkloadFile: &wlp,
	}

	t.Run("mounts workload run path", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(pathWl, "/work-dir")
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlp),
					Target: "/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script",
				},
			},
		}, cmd)
	})

	argsWl := workload
	argsWl.Args = []bass.Value{wlp, bass.DirPath{"data"}}

	t.Run("paths in args", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(argsWl, "/work-dir")
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run", "/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script", "./data/"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlp),
					Target: "/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script",
				},
			},
		}, cmd)
	})

	stdinWl := workload
	stdinWl.Stdin = []bass.Value{
		bass.Bindings{
			"context": wlp,
			"out":     bass.DirPath{"data"},
		}.Scope(),
		bass.Int(42),
	}

	t.Run("paths in stdin", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(stdinWl, "/work-dir")
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
			Stdin: []bass.Value{
				bass.Bindings{
					"context": bass.String("/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script"),
					"out":     bass.String("./data/"),
				}.Scope(),
				bass.Int(42),
			},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlp),
					Target: "/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script",
				},
			},
		}, cmd)
	})

	envWlp := wlp
	envWlp.Path = bass.FileOrDirPath{
		File: &bass.FilePath{"env-file"},
	}

	envWlpWl := workload
	envWlpWl.Env = bass.Bindings{
		"INPUT": envWlp}.Scope()

	t.Run("workload paths in env", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(envWlpWl, "/work-dir")
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
			Env:  []string{"INPUT=/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/env-file"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(envWlp),
					Target: "/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/env-file",
				},
			},
		}, cmd)
	})

	envArgWl := workload
	envArgWl.Env = bass.Bindings{
		"FOO": bass.Bindings{
			"arg": bass.NewList(
				bass.String("foo="),
				bass.DirPath{"some/dir"},
				bass.String("!"),
			)}.Scope()}.Scope()

	t.Run("concatenating args", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(envArgWl, "/work-dir")
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
			Env:  []string{"FOO=foo=./some/dir/!"},
		}, cmd)
	})

	dirWlp := wlp
	dirWlp.Path = bass.FileOrDirPath{
		Dir: &bass.DirPath{"dir-dir"},
	}

	dirWlpWl := workload
	dirWlpWl.Dir = &bass.RunDirPath{
		WorkloadDir: &dirWlp,
	}

	t.Run("workload path as dir", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(dirWlpWl, "/work-dir")
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
			Dir:  strptr("/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/dir-dir/"),
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(dirWlp),
					Target: "/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/dir-dir/",
				},
			},
		}, cmd)
	})

	dupeWl := workload
	dupeWl.Path = bass.RunPath{
		WorkloadFile: &wlp,
	}
	dupeWl.Args = []bass.Value{wlp}
	dupeWl.Stdin = []bass.Value{wlp}
	dupeWl.Env = bass.Bindings{"INPUT": wlp}.Scope()
	dupeWl.Dir = &bass.RunDirPath{
		WorkloadDir: &dirWlp,
	}

	t.Run("does not mount same path twice", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(dupeWl, "/work-dir")
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args:  []string{"/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script", "/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script"},
			Stdin: []bass.Value{bass.String("/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script")},
			Env:   []string{"INPUT=/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script"},
			Dir:   strptr("/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/dir-dir/"),
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlp),
					Target: "/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script",
				},
				{
					Source: bass.WorkloadPathSource(dirWlp),
					Target: "/work-dir/52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/dir-dir/",
				},
			},
		}, cmd)
	})

	mountsWl := workload
	mountsWl.Mounts = []bass.RunMount{
		{
			Source: bass.WorkloadPathSource(wlp),
			Target: bass.FileOrDirPath{
				Dir: &bass.DirPath{"dir"},
			},
		},
	}

	t.Run("mounts", func(t *testing.T) {
		cmd, err := runtimes.NewCommand(mountsWl, "/work-dir")
		require.NoError(t, err)
		require.Equal(t, runtimes.Command{
			Args: []string{"run"},
			Mounts: []runtimes.CommandMount{
				{
					Source: bass.WorkloadPathSource(wlp),
					Target: "./dir/",
				},
			},
		}, cmd)
	})
}

func strptr(s string) *string {
	return &s
}
