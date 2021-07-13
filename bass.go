package bass

import (
	"errors"
	"io"
	"os"
)

type RuntimeState struct {
	Stderr io.Writer
}

func NewStandardEnv() *Env {
	return NewEnv(ground)
}

func NewRuntimeEnv(state RuntimeState) *Env {
	std := NewStandardEnv()

	dispatch := LoadRuntimeDispatch(state)

	std.Set("run",
		Applicative{Op("run", dispatch.Run)},
		`run a workload`,
		`A workload is a command to run on some platform.`,
		`Structurally, a workload is an object? with a :platform and a :command. A workload's platform is used to select a runtime to run the command.`,
		`To construct a workload to run natively on the host platform, use ($) to pass string arguments on the commandline, or use command paths (.foo) or file paths (./foo) to pass arbitrary arguments on stdin.`,
		`Commands must describe all inputs which may change the result of the command: arguments, stdin, environment variables, container image, etc.`,
		`Runtimes other than the native runtime may be used to run a command in an isolated or remote environment, such as a container or a cluster of worker machines.`,
	)

	return NewEnv(std)
}

func EvalFile(env *Env, filePath string) (Value, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return EvalReader(env, file)
}

func EvalReader(e *Env, r io.Reader) (Value, error) {
	reader := NewReader(r)

	var res Value
	for {
		val, err := reader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		rdy := val.Eval(e, Identity)

		res, err = Trampoline(rdy)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func Trampoline(val Value) (Value, error) {
	for {
		var cont ReadyCont
		if err := val.Decode(&cont); err != nil {
			return val, nil
		}

		var err error
		val, err = cont.Go()
		if err != nil {
			return nil, err
		}
	}
}
