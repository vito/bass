package runtimes

import (
	"bytes"
	"context"
	"io"

	"github.com/concourse/go-archive/tarfs"
	"github.com/vito/bass"
)

func NewEnv(runtime Runtime) *bass.Env {
	env := bass.NewStandardEnv()

	env.Set("name",
		bass.Func("name", (bass.Workload).SHA1),
		`returns a workload's name`,
		`Workload names are generated from the workload content.`,
		`Their structure should not be relied upon and it may change at any time.`,
	)

	env.Set("path",
		bass.Func("path", func(workload bass.Workload, path bass.FileOrDirPath) bass.WorkloadPath {
			return bass.WorkloadPath{
				Workload: workload,
				Path:     path,
			}
		}),
		`returns a path within a workload`,
	)

	env.Set("run",
		bass.Func("run", func(ctx context.Context, workload bass.Workload) (*bass.Source, error) {
			err := runtime.Run(ctx, workload)
			if err != nil {
				return nil, err
			}

			buf := new(bytes.Buffer)
			err = runtime.Response(ctx, buf, workload)
			if err != nil {
				return nil, err
			}

			name, err := workload.SHA1()
			if err != nil {
				return nil, err
			}

			return bass.NewSource(bass.NewJSONSource(name, buf)), nil
		}),
		`run a workload`,
		`A workload is a command to run on some platform.`,
		`Structurally, a workload is an object? with a :platform and a :command. A workload's platform is used to select a runtime to run the command.`,
		`To construct a workload to run natively on the host platform, use ($) to pass string arguments on the commandline, or use command paths (.foo) or file paths (./foo) to pass arbitrary arguments on stdin.`,
		`Commands must describe all inputs which may change the result of the command: arguments, stdin, environment variables, container image, etc.`,
		`Runtimes other than the native runtime may be used to run a command in an isolated or remote environment, such as a container or a cluster of worker machines.`,
	)

	env.Set("export",
		bass.Func("export", func(ctx context.Context, path bass.WorkloadPath, dest bass.DirPath) error {
			r, w := io.Pipe()
			go func() {
				w.CloseWithError(runtime.Export(ctx, w, path.Workload, path.Path.FilesystemPath()))
			}()

			return tarfs.Extract(r, dest.FromSlash())
		}),
	)

	return bass.NewEnv(env)
}
