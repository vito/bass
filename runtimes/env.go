package runtimes

import (
	"bytes"
	"context"

	"github.com/vito/bass"
)

type RunState struct {
	Dir    bass.Path
	Args   bass.List
	Stdin  *bass.Source
	Stdout *bass.Sink
}

func NewEnv(pool *Pool, state RunState) *bass.Env {
	env := bass.NewStandardEnv()

	env.Set("*dir*",
		state.Dir,
		`working directory`,
		`This value is always set to the directory containing the file being run.`,
		`It can and should be used to load sibling/child paths, e.g. *dir*/foo to load the 'foo.bass' file in the same directory as the current file.`)

	env.Set("*args*",
		state.Args,
		`command line arguments`)

	env.Set("*stdin*",
		state.Stdin,
		`standard input stream`)

	env.Set("*stdout*",
		state.Stdout,
		`standard output sink`)

	env.Set("load",
		bass.Func("load", func(ctx context.Context, workload bass.Workload) (*bass.Env, error) {
			err := pool.Run(ctx, workload)
			if err != nil {
				return nil, err
			}

			return pool.Env(ctx, workload)
		}))

	env.Set("run",
		bass.Func("run", func(ctx context.Context, workload bass.Workload) (*bass.Source, error) {
			err := pool.Run(ctx, workload)
			if err != nil {
				return nil, err
			}

			buf := new(bytes.Buffer)
			err = pool.Response(ctx, buf, workload)
			if err != nil {
				return nil, err
			}

			return bass.NewSource(bass.NewJSONSource(workload.String(), buf)), nil
		}),
		`run a workload`)

	env.Set("path",
		bass.Func("path", func(workload bass.Workload, path bass.FileOrDirPath) bass.WorkloadPath {
			return bass.WorkloadPath{
				Workload: workload,
				Path:     path,
			}
		}),
		`returns a path within a workload`)

	return bass.NewEnv(env)
}
