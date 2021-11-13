package runtimes

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-cmp/cmp"
	"github.com/vito/bass"
)

// Command is a helper type constructed by a runtime by Resolving a Thunk.
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

// CommandMount configures a thunk path to mount to the command's container.
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

// Resolve traverses the Thunk, resolving logical path values to their
// concrete paths in the container, and collecting the requisite mount points
// along the way.
func NewCommand(thunk bass.Thunk) (Command, error) {
	cmd := &Command{
		mounted: map[string]bool{},
	}

	var err error

	if thunk.Dir != nil {
		var cwd string
		err := cmd.resolveValue(thunk.Dir.ToValue(), &cwd)
		if err != nil {
			return Command{}, fmt.Errorf("resolve wd: %w", err)
		}

		cmd.Dir = &cwd
	}

	if thunk.Entrypoint != nil {
		cmd.Entrypoint, err = cmd.resolveArgs(thunk.Entrypoint)
		if err != nil {
			return Command{}, fmt.Errorf("resolve entrypoint: %w", err)
		}
	}

	var path string
	err = cmd.resolveValue(thunk.Path.ToValue(), &path)
	if err != nil {
		return Command{}, fmt.Errorf("resolve path: %w", err)
	}

	cmd.Args = []string{path}

	if thunk.Args != nil {
		vals, err := cmd.resolveArgs(thunk.Args)
		if err != nil {
			return Command{}, fmt.Errorf("resolve args: %w", err)
		}

		cmd.Args = append(cmd.Args, vals...)
	}

	if thunk.Env != nil {
		// TODO: using a map here may mean nondeterminism
		err := thunk.Env.Each(func(name bass.Symbol, v bass.Value) error {
			var val string
			err := cmd.resolveValue(v, &val)
			if err != nil {
				return fmt.Errorf("resolve env %s: %w", name, err)
			}

			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", name.JSONKey(), val))
			return nil
		})
		if err != nil {
			return Command{}, err
		}
	}

	if thunk.Stdin != nil {
		cmd.Stdin, err = cmd.resolveValues(thunk.Stdin)
		if err != nil {
			return Command{}, fmt.Errorf("resolve stdin: %w", err)
		}
	}

	if thunk.Mounts != nil {
		for _, m := range thunk.Mounts {
			cmd.Mounts = append(cmd.Mounts, CommandMount{
				Source: m.Source,
				Target: m.Target.FilesystemPath().FromSlash(),
			})
		}
	}

	cmd.mounted = nil

	return *cmd, nil
}

func (cmd Command) Equal(other Command) bool {
	return cmp.Equal(cmd.Entrypoint, other.Entrypoint) &&
		cmp.Equal(cmd.Args, other.Args) &&
		cmp.Equal(cmd.Stdin, other.Stdin) &&
		cmp.Equal(cmd.Env, other.Env) &&
		cmp.Equal(cmd.Dir, other.Dir) &&
		cmp.Equal(cmd.Mounts, other.Mounts)
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

	var artifact bass.ThunkPath
	if err := val.Decode(&artifact); err == nil {
		// TODO: it might be worth mounting the entire artifact directory instead
		name, err := artifact.Thunk.SHA1()
		if err != nil {
			return err
		}

		fsp := artifact.Path.FilesystemPath()

		target, err := bass.FileOrDirPath{
			Dir: &bass.DirPath{Path: name},
		}.Extend(fsp)
		if err != nil {
			return err
		}

		targetPath := target.FilesystemPath().FromSlash()

		if !cmd.mounted[targetPath] {
			cmd.Mounts = append(cmd.Mounts, CommandMount{
				Source: &bass.MountSourceEnum{
					ThunkPath: &artifact,
				},
				Target: targetPath,
			})

			cmd.mounted[targetPath] = true
		}

		pathValue := targetPath
		if cmd.Dir != nil {
			for dir := filepath.Dir(*cmd.Dir); dir != "."; dir = filepath.Dir(dir) {
				pathValue = filepath.Join("..", pathValue)
				if fsp.IsDir() {
					pathValue += string(os.PathSeparator)
				}
			}
		}

		return bass.String(pathValue).Decode(dest)
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
