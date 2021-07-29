package bass

import (
	"context"
	"io"
)

type Runtime interface {
	Run(context.Context, Workload) (PipeSource, error)
}

type Dispatch struct {
	Runtimes []RuntimeAssoc
}

type RuntimeState struct {
	Stderr io.Writer

	Args []Value
}

func NewRuntimeEnv(state RuntimeState) *Env {
	dispatch := LoadRuntimeDispatch(state)

	env := NewStandardEnv()

	env.Set("*args*",
		NewList(state.Args...),
		`arguments passed to the script on the command line`,
		`String arguments that parse as paths are converted to paths referring to their underlying file or directory.`)

	env.Set("run",
		Wrapped{Op("run", dispatch.Run)},
		`run a workload`,
		`A workload is a command to run on some platform.`,
		`Structurally, a workload is an object? with a :platform and a :command. A workload's platform is used to select a runtime to run the command.`,
		`To construct a workload to run natively on the host platform, use ($) to pass string arguments on the commandline, or use command paths (.foo) or file paths (./foo) to pass arbitrary arguments on stdin.`,
		`Commands must describe all inputs which may change the result of the command: arguments, stdin, environment variables, container image, etc.`,
		`Runtimes other than the native runtime may be used to run a command in an isolated or remote environment, such as a container or a cluster of worker machines.`,
	)

	return NewEnv(env)
}

type RuntimeAssoc struct {
	Platform Object  `json:"platform"`
	Runtime  Runtime `json:"runtime"`
}

func (dispatch Dispatch) Run(ctx context.Context, workload Workload) (PipeSource, error) {
	for _, runtime := range dispatch.Runtimes {
		if workload.Platform.Equal(runtime.Platform) {
			return runtime.Runtime.Run(ctx, workload)
		}
	}

	return nil, ErrNoRuntime
}

func LoadRuntimeDispatch(state RuntimeState) *Dispatch {
	// TODO: load runtimes.json
	return &Dispatch{
		Runtimes: []RuntimeAssoc{},
	}
}
