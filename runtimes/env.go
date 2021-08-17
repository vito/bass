package runtimes

import (
	"bytes"
	"context"

	"github.com/vito/bass"
)

func NewEnv(pool *Pool) *bass.Env {
	env := bass.NewStandardEnv()

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

			name, err := workload.SHA1()
			if err != nil {
				return nil, err
			}

			return bass.NewSource(bass.NewJSONSource(name, buf)), nil
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
