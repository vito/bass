package runtimes

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/google/go-cmp/cmp"
	"github.com/vito/bass/pkg/bass"
)

// Command is a helper type constructed by a runtime by Resolving a Thunk.
//
// It contains the direct values to be provided for the process running in the
// container.
type Command struct {
	Args  []string `json:"args"`
	Stdin []byte   `json:"stdin"`
	Env   []string `json:"env"`
	Dir   *string  `json:"dir"`

	Mounts []CommandMount `json:"-"` // doesn't need to be marshaled

	mounted map[string]bool
}

// CommandMount configures a thunk path to mount to the command's container.
type CommandMount struct {
	Source bass.ThunkMountSource
	Target string
}

// StrThunk contains a list of values to be resolved to strings and
// concatenated together to form a single string.
//
// It is used to concatenate path thunks with literal strings.
type StrThunk struct {
	Values bass.List `json:"str"`
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

	var path string
	err = cmd.resolveValue(thunk.Cmd.ToValue(), &path)
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
		err := thunk.Env.Each(func(name bass.Symbol, v bass.Value) error {
			val, err := cmd.resolveStr(v)
			if err != nil {
				return fmt.Errorf("resolve env %s: %w", name, err)
			}

			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", name.JSONKey(), val))
			return nil
		})
		if err != nil {
			return Command{}, err
		}

		sort.Strings(cmd.Env)
	}

	if thunk.Stdin != nil {
		stdin, err := cmd.resolveValues(thunk.Stdin)
		if err != nil {
			return Command{}, fmt.Errorf("resolve stdin: %w", err)
		}

		stdinBuf := new(bytes.Buffer)
		enc := bass.NewEncoder(stdinBuf)
		for _, val := range stdin {
			err := enc.Encode(val)
			if err != nil {
				return Command{}, fmt.Errorf("encode stdin: %w", err)
			}
		}

		cmd.Stdin = stdinBuf.Bytes()
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
	return cmp.Equal(cmd.Args, other.Args) &&
		cmp.Equal(cmd.Stdin, other.Stdin) &&
		cmp.Equal(cmd.Env, other.Env) &&
		cmp.Equal(cmd.Dir, other.Dir) &&
		cmp.Equal(cmd.Mounts, other.Mounts)
}

func (cmd *Command) resolveStr(val bass.Value) (string, error) {
	var str string

	var concat bass.List
	if err := val.Decode(&concat); err == nil {
		err := cmd.resolveArg(concat, &str)
		if err != nil {
			return "", fmt.Errorf("concat: %w", err)
		}
	} else if err := cmd.resolveValue(val, &str); err != nil {
		return "", err
	}

	return str, nil
}

func (cmd *Command) resolveArgs(list []bass.Value) ([]string, error) {
	var args []string
	for i, v := range list {
		arg, err := cmd.resolveStr(v)
		if err != nil {
			return nil, fmt.Errorf("arg %d: %w", i, err)
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

func (cmd *Command) resolveValue(val bass.Value, dest any) error {
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
		name, err := artifact.Thunk.SHA256()
		if err != nil {
			return err
		}

		target, err := bass.DirPath{Path: name}.Extend(artifact.Path.FilesystemPath())
		if err != nil {
			return err
		}

		fsp := target.(bass.FilesystemPath)

		targetPath := fsp.FromSlash()
		if !cmd.mounted[targetPath] {
			cmd.Mounts = append(cmd.Mounts, CommandMount{
				Source: bass.ThunkMountSource{
					ThunkPath: &artifact,
				},
				Target: targetPath,
			})

			cmd.mounted[targetPath] = true
		}

		return bass.String(cmd.rel(fsp)).Decode(dest)
	}

	var host bass.HostPath
	if err := val.Decode(&host); err == nil {
		target, err := bass.DirPath{
			Path: hash(host.ContextDir),
		}.Extend(host.Path.FilesystemPath())
		if err != nil {
			return err
		}

		fsp := target.(bass.FilesystemPath)

		targetPath := fsp.FromSlash()
		if !cmd.mounted[targetPath] {
			cmd.Mounts = append(cmd.Mounts, CommandMount{
				Source: bass.ThunkMountSource{
					HostPath: &host,
				},
				Target: targetPath,
			})

			cmd.mounted[targetPath] = true
		}

		return bass.String(cmd.rel(fsp)).Decode(dest)
	}

	var embedPath *bass.FSPath
	if err := val.Decode(&embedPath); err == nil {
		sha2, err := embedPath.SHA256()
		if err != nil {
			return err
		}

		target, err := bass.DirPath{
			Path: sha2,
		}.Extend(embedPath.Path.FilesystemPath())
		if err != nil {
			return err
		}

		fsp := target.(bass.FilesystemPath)

		targetPath := fsp.FromSlash()
		if !cmd.mounted[targetPath] {
			cmd.Mounts = append(cmd.Mounts, CommandMount{
				Source: bass.ThunkMountSource{
					FSPath: embedPath,
				},
				Target: targetPath,
			})

			cmd.mounted[targetPath] = true
		}

		return bass.String(cmd.rel(fsp)).Decode(dest)
	}

	var secret bass.Secret
	if err := val.Decode(&secret); err == nil {
		shhhhh := secret.Reveal()
		if shhhhh == nil {
			return fmt.Errorf("missing secret: %s", secret.Name)
		}

		return bass.String(shhhhh).Decode(dest)
	}

	return val.Decode(dest)
}

func (cmd *Command) rel(workRelPath bass.FilesystemPath) string {
	if cmd.Dir == nil {
		return workRelPath.FromSlash()
	}

	var cwdRelPath = workRelPath.FromSlash()
	for dir := filepath.Dir(*cmd.Dir); dir != "."; dir = filepath.Dir(dir) {
		cwdRelPath = filepath.Join("..", cwdRelPath)
	}

	if workRelPath.IsDir() {
		cwdRelPath += string(os.PathSeparator)
	}

	return cwdRelPath
}

func (cmd *Command) resolveArg(vals bass.List, dest any) error {
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
