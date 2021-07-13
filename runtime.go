package bass

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// NativePlatform is a platform that will run workloads directly on the host
// environment.
var NativePlatform = Object{
	"native": Bool(true),
}

// NativeCommand is a command to run natively on the host machine.
type NativeCommand struct {
	// Path is either a string or a path value specifying the command or file to run.
	Path Value `bass:"path"`

	// Args is a list of string or path values, including argv[0] as the program
	// being run.
	Args List `bass:"args" optional:"true"`

	// Stdin is a fixed list of values to write as a JSON stream on stdin.
	//
	// This is distinct from a stream interface; it is a finite part of the
	// request so that it may be used to form a cache key.
	Stdin Value `bass:"stdin" optional:"true"`

	// Env is a map of environment variables to set for the workload.
	Env Object `bass:"env" optional:"true"`

	// From is a string or path value specifying the working directory the process should be run within.
	Dir Value `bass:"dir" optional:"true"`
}

type Runtime interface {
	Run(Cont, *Env, Value, ...Combiner) ReadyCont
}

type Dispatch struct {
	Runtimes []RuntimeAssoc
}

type RuntimeAssoc struct {
	Platform Object  `json:"platform"`
	Runtime  Runtime `json:"runtime"`
}

func (dispatch Dispatch) Run(cont Cont, env *Env, workload Workload, cbOptional ...Combiner) ReadyCont {
	for _, runtime := range dispatch.Runtimes {
		if workload.Platform.Equal(runtime.Platform) {
			return runtime.Runtime.Run(cont, env, workload.Command, cbOptional...)
		}
	}

	return cont.Call(nil, ErrNoRuntime)
}

func LoadRuntimeDispatch(state RuntimeState) *Dispatch {
	// TODO: load runtimes.json
	return &Dispatch{
		Runtimes: []RuntimeAssoc{
			{
				Platform: NativePlatform,
				Runtime: Native{
					Stderr: state.Stderr,
				},
			},
		},
	}
}

type Native struct {
	Stderr io.Writer
}

func (runtime Native) Run(cont Cont, env *Env, val Value, cbOptional ...Combiner) ReadyCont {
	var command NativeCommand
	err := val.Decode(&command)
	if err != nil {
		return cont.Call(nil, err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return cont.Call(nil, err)
	}

	var path string

	var path_ Path
	if err := command.Path.Decode(&path_); err == nil {
		path, err = path_.Resolve(cwd)
		if err != nil {
			return cont.Call(nil, err)
		}
	} else if err := command.Path.Decode(&path); err != nil {
		return cont.Call(nil, err)
	}

	var args []string
	if command.Args != nil {
		err = Each(command.Args, func(val Value) error {
			var arg string

			var path_ Path
			if err := val.Decode(&path_); err == nil {
				arg, err = path_.Resolve(cwd)
				if err != nil {
					return err
				}
			} else if err := val.Decode(&arg); err != nil {
				return err
			}

			args = append(args, arg)

			return nil
		})
		if err != nil {
			return cont.Call(nil, err)
		}
	}

	cmd := exec.Command(path, args...)
	cmd.Stderr = runtime.Stderr

	if len(cbOptional) == 0 {
		cmd.Stdout = runtime.Stderr

		err := cmd.Run()
		if err != nil {
			return cont.Call(nil, err)
		}

		return cont.Call(Null{}, nil)
	}

	if len(cbOptional) > 1 {
		return cont.Call(nil, fmt.Errorf("TODO: extra callback supplied"))
	}

	cb := cbOptional[0]

	out, err := cmd.StdoutPipe()
	if err != nil {
		return cont.Call(nil, err)
	}

	source := &Source{
		NewJSONSource("cmd", out),
	}

	err = cmd.Start()
	if err != nil {
		return cont.Call(nil, err)
	}

	return cb.Call(NewList(source), env, Continue(func(res Value) Value {
		err := cmd.Wait()
		if err != nil {
			return cont.Call(nil, err)
		}

		return cont.Call(res, nil)
	}))
}
