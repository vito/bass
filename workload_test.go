package bass_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestWorkloadName(t *testing.T) {
	// use an object with a ton of keys to test stable order when hashing
	manyKeys := bass.Object{}
	for i := 0; i < 100; i++ {
		manyKeys[bass.Keyword(fmt.Sprintf("key-%d", i))] = bass.Int(i)
	}

	workload := bass.Workload{
		Platform: manyKeys,
		Path: bass.RunPath{
			File: &bass.FilePath{"run"},
		},
		Env: manyKeys,
	}

	name, err := workload.Name()
	require.NoError(t, err)

	// this is a bit silly, but it's deterministic, and we need to make sure it's
	// always the same value
	require.Equal(t, "d8e13a4277f7532e6c8a6eb162fbdb2e592324a7", name)
}

func TestWorkloadResolve(t *testing.T) {
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
		cmd, err := workload.Resolve()
		require.NoError(t, err)
		require.Equal(t, bass.Command{
			Args: []string{"run"},
		}, cmd)
	})

	entrypointWl := workload
	entrypointWl.Entrypoint = []bass.Value{bass.CommandPath{"bash"}}

	t.Run("command in entrypoint", func(t *testing.T) {
		cmd, err := entrypointWl.Resolve()
		require.NoError(t, err)
		require.Equal(t, bass.Command{
			Entrypoint: []string{"bash"},
			Args:       []string{"run"},
		}, cmd)
	})

	fileWl := workload
	fileWl.Path = bass.RunPath{
		File: &bass.FilePath{"run"},
	}

	t.Run("file run path", func(t *testing.T) {
		cmd, err := fileWl.Resolve()
		require.NoError(t, err)
		require.Equal(t, bass.Command{
			Args: []string{"./run"},
		}, cmd)
	})

	pathWl := workload
	pathWl.Path = bass.RunPath{
		WorkloadFile: &wlp,
	}

	t.Run("mounts workload run path", func(t *testing.T) {
		cmd, err := pathWl.Resolve()
		require.NoError(t, err)
		require.Equal(t, bass.Command{
			Args: []string{"./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script"},
			Mounts: []bass.CommandMount{
				{
					Source: wlp,
					Target: "./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script",
				},
			},
		}, cmd)
	})

	argsWl := workload
	argsWl.Args = []bass.Value{wlp, bass.DirPath{"data"}}

	t.Run("paths in args", func(t *testing.T) {
		cmd, err := argsWl.Resolve()
		require.NoError(t, err)
		require.Equal(t, bass.Command{
			Args: []string{"run", "./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script", "./data/"},
			Mounts: []bass.CommandMount{
				{
					Source: wlp,
					Target: "./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script",
				},
			},
		}, cmd)
	})

	stdinWl := workload
	stdinWl.Stdin = []bass.Value{
		bass.Object{
			"context": wlp,
			"out":     bass.DirPath{"data"},
		},
		bass.Int(42),
	}

	t.Run("paths in stdin", func(t *testing.T) {
		cmd, err := stdinWl.Resolve()
		require.NoError(t, err)
		require.Equal(t, bass.Command{
			Args: []string{"run"},
			Stdin: []bass.Value{
				bass.Object{
					"context": bass.String("./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script"),
					"out":     bass.String("./data/"),
				},
				bass.Int(42),
			},
			Mounts: []bass.CommandMount{
				{
					Source: wlp,
					Target: "./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script",
				},
			},
		}, cmd)
	})

	envWlp := wlp
	envWlp.Path = bass.FileOrDirPath{
		File: &bass.FilePath{"env-file"},
	}

	envWlpWl := workload
	envWlpWl.Env = bass.Object{
		"INPUT": envWlp,
	}

	t.Run("workload paths in env", func(t *testing.T) {
		cmd, err := envWlpWl.Resolve()
		require.NoError(t, err)
		require.Equal(t, bass.Command{
			Args: []string{"run"},
			Env:  []string{"INPUT=./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/env-file"},
			Mounts: []bass.CommandMount{
				{
					Source: envWlp,
					Target: "./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/env-file",
				},
			},
		}, cmd)
	})

	envArgWl := workload
	envArgWl.Env = bass.Object{
		"FOO": bass.Object{
			"arg": bass.NewList(
				bass.String("foo="),
				bass.DirPath{"some/dir"},
				bass.String("!"),
			),
		},
	}

	t.Run("concatenating args", func(t *testing.T) {
		cmd, err := envArgWl.Resolve()
		require.NoError(t, err)
		require.Equal(t, bass.Command{
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
		cmd, err := dirWlpWl.Resolve()
		require.NoError(t, err)
		require.Equal(t, bass.Command{
			Args: []string{"run"},
			Dir:  strptr("./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/dir-dir/"),
			Mounts: []bass.CommandMount{
				{
					Source: dirWlp,
					Target: "./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/dir-dir/",
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
	dupeWl.Env = bass.Object{"INPUT": wlp}
	dupeWl.Dir = &bass.RunDirPath{
		WorkloadDir: &dirWlp,
	}

	t.Run("does not mount same path twice", func(t *testing.T) {
		cmd, err := dupeWl.Resolve()
		require.NoError(t, err)
		require.Equal(t, bass.Command{
			Args:  []string{"./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script", "./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script"},
			Stdin: []bass.Value{bass.String("./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script")},
			Env:   []string{"INPUT=./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script"},
			Dir:   strptr("./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/dir-dir/"),
			Mounts: []bass.CommandMount{
				{
					Source: wlp,
					Target: "./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/script",
				},
				{
					Source: dirWlp,
					Target: "./52d9caa2609a8b07ffc6d82b2ed96026fa8e5fbf/dir-dir/",
				},
			},
		}, cmd)
	})

	mountsWl := workload
	mountsWl.Mounts = []bass.RunMount{
		{
			Source: wlp,
			Target: bass.FileOrDirPath{
				Dir: &bass.DirPath{"dir"},
			},
		},
	}

	t.Run("mounts", func(t *testing.T) {
		cmd, err := mountsWl.Resolve()
		require.NoError(t, err)
		require.Equal(t, bass.Command{
			Args: []string{"run"},
			Mounts: []bass.CommandMount{
				{
					Source: wlp,
					Target: "./dir/",
				},
			},
		}, cmd)
	})
}

func strptr(s string) *string {
	return &s
}
