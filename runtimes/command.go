package runtimes

import (
	"fmt"

	"github.com/vito/bass"
)

// Command is a helper type constructed by a runtime by Resolving a Workload.
//
// It contains the direct values to be provided for the process running in the
// container.
type Command struct {
	Entrypoint []string
	Args       []string
	Stdin      []bass.Value
	Env        []string
	Dir        *string

	Mounts  []CommandMount
	mounted map[string]bool
}

// CommandMount configures a workload path to mount to the command's container.
type CommandMount struct {
	Source *bass.MountSourceEnum `json:"source"`
	Target string                `json:"target"`
}

// Arg is a sequence of values to be resolved and concatenated together to form
// a single string argument.
//
// It is used to concatenate logical path values with literal strings.
type Arg struct {
	Values bass.List `json:"arg"`
}

// Resolve traverses the Workload, resolving logical path values to their
// concrete paths in the container, and collecting the requisite mount points
// along the way.
func NewCommand(workload bass.Workload) (Command, error) {
	cmd := &Command{
		mounted: map[string]bool{},
	}

	var err error

	if workload.Entrypoint != nil {
		cmd.Entrypoint, err = cmd.resolveArgs(workload.Entrypoint)
		if err != nil {
			return Command{}, fmt.Errorf("resolve entrypoint: %w", err)
		}
	}

	var path string
	err = cmd.resolveValue(workload.Path.ToValue(), &path)
	if err != nil {
		return Command{}, fmt.Errorf("resolve path: %w", err)
	}

	cmd.Args = []string{path}

	if workload.Args != nil {
		vals, err := cmd.resolveArgs(workload.Args)
		if err != nil {
			return Command{}, fmt.Errorf("resolve args: %w", err)
		}

		cmd.Args = append(cmd.Args, vals...)
	}

	if workload.Env != nil {
		// TODO: using a map here may mean nondeterminism
		for name, v := range workload.Env {
			var val string
			err := cmd.resolveValue(v, &val)
			if err != nil {
				return Command{}, fmt.Errorf("resolve env %s: %w", name, err)
			}

			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", name.JSONKey(), val))
		}
	}

	if workload.Dir != nil {
		var cwd string
		err := cmd.resolveValue(workload.Dir.ToValue(), &cwd)
		if err != nil {
			return Command{}, fmt.Errorf("resolve wd: %w", err)
		}

		cmd.Dir = &cwd
	}

	if workload.Stdin != nil {
		cmd.Stdin, err = cmd.resolveValues(workload.Stdin)
		if err != nil {
			return Command{}, fmt.Errorf("resolve stdin: %w", err)
		}
	}

	if workload.Mounts != nil {
		for _, m := range workload.Mounts {
			cmd.Mounts = append(cmd.Mounts, CommandMount{
				Source: m.Source,
				Target: m.Target.FilesystemPath().FromSlash(),
			})
		}
	}

	cmd.mounted = nil
	return *cmd, nil
}

func (cmd *Command) resolveArgs(list []bass.Value) ([]string, error) {
	var args []string
	for _, v := range list {
		var arg string
		err := cmd.resolveValue(v, &arg)
		if err != nil {
			return nil, err
		}

		args = append(args, arg)
	}

	return args, nil
}

func (cmd *Command) resolveValues(list []bass.Value) ([]bass.Value, error) {
	var vals []bass.Value
	for _, v := range list {
		resolved, err := bass.Resolve(v, func(v2 bass.Value) (bass.Value, error) {
			var val bass.Value
			err := cmd.resolveValue(v2, &val)
			if err != nil {
				return nil, err
			}

			return val, nil
		})
		if err != nil {
			return nil, err
		}

		vals = append(vals, resolved)
	}

	return vals, nil
}

func (cmd *Command) resolveValue(val bass.Value, dest interface{}) error {
	var arg Arg
	if err := val.Decode(&arg); err == nil {
		return cmd.resolveArg(arg.Values, dest)
	}

	var file bass.FilePath
	if err := val.Decode(&file); err == nil {
		return bass.String(file.FromSlash()).Decode(dest)
	}

	var dir bass.DirPath
	if err := val.Decode(&dir); err == nil {
		return bass.String(dir.FromSlash()).Decode(dest)
	}

	var cmdp bass.CommandPath
	if err := val.Decode(&cmdp); err == nil {
		return bass.String(cmdp.Command).Decode(dest)
	}

	var artifact bass.WorkloadPath
	if err := val.Decode(&artifact); err == nil {
		// TODO: it might be worth mounting the entire artifact directory instead
		name, err := artifact.Workload.SHA1()
		if err != nil {
			return err
		}

		target, err := bass.FileOrDirPath{
			Dir: &bass.DirPath{Path: name},
		}.Extend(artifact.Path.FilesystemPath())
		if err != nil {
			return err
		}

		path := target.FilesystemPath().FromSlash()

		if !cmd.mounted[path] {
			cmd.Mounts = append(cmd.Mounts, CommandMount{
				Source: &bass.MountSourceEnum{
					WorkloadPath: &artifact,
				},
				Target: path,
			})

			cmd.mounted[path] = true
		}

		return bass.String(path).Decode(dest)
	}

	return val.Decode(dest)
}

func (cmd *Command) resolveArg(vals bass.List, dest interface{}) error {
	var res string
	err := bass.Each(vals, func(v bass.Value) error {
		var val string
		err := cmd.resolveValue(v, &val)
		if err != nil {
			return err
		}

		res += val

		return nil
	})
	if err != nil {
		return err
	}

	return bass.String(res).Decode(dest)
}
