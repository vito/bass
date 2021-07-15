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
	Stdin List `bass:"stdin" optional:"true"`

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

type RuntimeState struct {
	Stderr io.Writer
}

func NewRuntimeEnv(state RuntimeState) *Env {
	dispatch := LoadRuntimeDispatch(state)

	env := NewStandardEnv()

	for _, lib := range []string{
		"std/commands.bass",
	} {
		file, err := std.Open(lib)
		if err != nil {
			panic(err)
		}

		_, err = EvalReader(env, file)
		if err != nil {
			panic(err)
		}

		file.Close()
	}

	env.Set("run",
		Applicative{Op("run", dispatch.Run)},
		`run a workload`,
		`A workload is a command to run on some platform.`,
		`Structurally, a workload is an object? with a :platform and a :command. A workload's platform is used to select a runtime to run the command.`,
		`To construct a workload to run natively on the host platform, use ($) to pass string arguments on the commandline, or use command paths (.foo) or file paths (./foo) to pass arbitrary arguments on stdin.`,
		`Commands must describe all inputs which may change the result of the command: arguments, stdin, environment variables, container image, etc.`,
		`Runtimes other than the native runtime may be used to run a command in an isolated or remote environment, such as a container or a cluster of worker machines.`,
	)

	env.Set("log",
		Func("log", func(v Value) {
			var str string
			if err := v.Decode(&str); err == nil {
				fmt.Fprintln(state.Stderr, str)
			} else {
				fmt.Fprintln(state.Stderr, v)
			}
		}),
		`write a string message or other arbitrary value to stderr`)

	return NewEnv(env)
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

func strOrPath(cwd string, val Value) (string, error) {
	var str string

	var path_ Path
	if err := val.Decode(&path_); err == nil {
		str, err = path_.Resolve(cwd)
		if err != nil {
			return "", err
		}
	} else if err := val.Decode(&str); err != nil {
		return "", err
	}

	return str, nil
}

func (runtime Native) Run(cont Cont, env *Env, val Value, cbOptional ...Combiner) ReadyCont {
	if len(cbOptional) > 1 {
		return cont.Call(nil, fmt.Errorf("TODO: extra callback supplied"))
	}

	var command NativeCommand
	err := val.Decode(&command)
	if err != nil {
		return cont.Call(nil, err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return cont.Call(nil, err)
	}

	path, err := strOrPath(cwd, command.Path)
	if err != nil {
		return cont.Call(nil, err)
	}

	var args []string
	if command.Args != nil {
		err = Each(command.Args, func(val Value) error {
			arg, err := strOrPath(cwd, val)
			if err != nil {
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

	var sink *JSONSink
	var closer io.Closer
	if command.Stdin != nil {
		in, err := cmd.StdinPipe()
		if err != nil {
			return cont.Call(nil, err)
		}

		closer = in

		sink = NewJSONSink("cmd", in)
	}

	var source *JSONSource
	if len(cbOptional) == 0 {
		cmd.Stdout = runtime.Stderr
	} else {
		out, err := cmd.StdoutPipe()
		if err != nil {
			return cont.Call(nil, err)
		}

		source = NewJSONSource("cmd", out)
	}

	cmd.Env = os.Environ()
	if command.Env != nil {
		for k, v := range command.Env {
			str, err := strOrPath(cwd, v)
			if err != nil {
				return cont.Call(nil, err)
			}

			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, str))
		}
	}

	if command.Dir != nil {
		cmd.Dir, err = strOrPath(cwd, command.Dir)
		if err != nil {
			return cont.Call(nil, err)
		}
	}

	err = cmd.Start()
	if err != nil {
		return cont.Call(nil, err)
	}

	if command.Stdin != nil {
		err := Each(command.Stdin, func(val Value) error {
			return sink.Emit(val)
		})
		if err != nil {
			return cont.Call(nil, err)
		}

		err = closer.Close()
		if err != nil {
			return cont.Call(nil, err)
		}
	}

	if len(cbOptional) == 1 {
		cb := cbOptional[0]

		return cb.Call(NewList(&Source{source}), env, Chain(cont, func(res Value) Value {
			err := cmd.Wait()
			if err != nil {
				return cont.Call(nil, err)
			}

			return cont.Call(res, nil)
		}))
	}

	err = cmd.Wait()
	if err != nil {
		return cont.Call(nil, err)
	}

	return cont.Call(Null{}, nil)
}
