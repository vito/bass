package runtimes

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/google/go-cmp/cmp"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/proto"
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
	Source *proto.ThunkMountSource
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
func NewCommand(thunk *proto.Thunk) (Command, error) {
	cmd := &Command{
		mounted: map[string]bool{},
	}

	if thunk.GetDir() != nil {
		var msg proto.Message
		if thunk.Dir.GetHostDir() != nil {
			msg = thunk.Dir.GetHostDir()
		} else if thunk.Dir.GetLocalDir() != nil {
			msg = thunk.Dir.GetLocalDir()
		} else if thunk.Dir.GetThunkDir() != nil {
			msg = thunk.Dir.GetThunkDir()
		}

		val, err := proto.NewValue(msg)
		if err != nil {
			return Command{}, fmt.Errorf("resolve wd: %w", err)
		}

		dir, err := cmd.resolveString(val)
		if err != nil {
			return Command{}, fmt.Errorf("resolve wd: %w", err)
		}

		cmd.Dir = &dir
	}

	var cmdMsg proto.Message
	switch x := thunk.Cmd.GetCmd().(type) {
	case *proto.ThunkCmd_CommandCmd:
		cmdMsg = x.CommandCmd
	case *proto.ThunkCmd_FileCmd:
		cmdMsg = x.FileCmd
	case *proto.ThunkCmd_FsCmd:
		cmdMsg = x.FsCmd
	case *proto.ThunkCmd_HostCmd:
		cmdMsg = x.HostCmd
	case *proto.ThunkCmd_ThunkCmd:
		cmdMsg = x.ThunkCmd
	default:
		return Command{}, fmt.Errorf("unsupported command type: %T", thunk.GetCmd())
	}

	cmdVal, err := proto.NewValue(cmdMsg)
	if err != nil {
		return Command{}, fmt.Errorf("resolve wd: %w", err)
	}

	argv0, err := cmd.resolveString(cmdVal)
	if err != nil {
		return Command{}, fmt.Errorf("resolve wd: %w", err)
	}

	cmd.Args = []string{argv0}

	if thunk.Args != nil {
		vals, err := cmd.resolveArgs(thunk.Args)
		if err != nil {
			return Command{}, fmt.Errorf("resolve args: %w", err)
		}

		cmd.Args = append(cmd.Args, vals...)
	}

	for _, bnd := range thunk.Env {
		val, err := cmd.resolveString(bnd.Value)
		if err != nil {
			return Command{}, fmt.Errorf("resolve env %s: %w", bnd.Name, err)
		}

		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", bnd.Name, val))
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
				Target: m.Target.FromSlash(),
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

func (cmd *Command) resolveArgs(list []*proto.Value) ([]string, error) {
	var args []string
	for _, v := range list {
		arg, err := cmd.resolveString(v)
		if err != nil {
			return nil, err
		}

		args = append(args, arg)
	}

	return args, nil
}

func (cmd *Command) resolveValues(list []*proto.Value) ([]*proto.Value, error) {
	var vals []*proto.Value
	for _, v := range list {
		resolved, err := proto.Resolve(v, func(v2 *proto.Value) (*proto.Value, error) {
			val, err := cmd.resolveValue(v2)
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

func (cmd *Command) resolveValue(val *proto.Value) (*proto.Value, error) {
	switch x := val.GetValue().(type) {
	case *proto.Value_FilePathValue:
		return proto.NewValue(&proto.String{Inner: x.FilePathValue.FromSlash()})
	case *proto.Value_DirPathValue:
		return proto.NewValue(&proto.String{Inner: x.DirPathValue.FromSlash()})
	case *proto.Value_CommandPathValue:
		return proto.NewValue(&proto.String{Inner: x.CommandPathValue.Command})
	case *proto.Value_ThunkPathValue:
		artifact := x.ThunkPathValue

		// TODO: it might be worth mounting the entire artifact directory instead
		name, err := artifact.Thunk.SHA256()
		if err != nil {
			return nil, err
		}

		target := &proto.FilesystemPath{}
		if artifact.Path.GetDir() != nil {
			target.Path = &proto.FilesystemPath_Dir{
				Dir: &proto.DirPath{
					Path: path.Join(name, artifact.Path.GetDir().Path),
				},
			}
		} else {
			target.Path = &proto.FilesystemPath_File{
				File: &proto.FilePath{
					Path: path.Join(name, artifact.Path.GetFile().Path),
				},
			}
		}

		targetPath := target.FromSlash()
		if !cmd.mounted[targetPath] {
			cmd.Mounts = append(cmd.Mounts, CommandMount{
				Source: &proto.ThunkMountSource{
					Source: &proto.ThunkMountSource_ThunkSource{
						ThunkSource: artifact,
					},
				},
				Target: targetPath,
			})

			cmd.mounted[targetPath] = true
		}

		return proto.NewValue(&proto.String{Inner: cmd.rel(target)})
	case *proto.Value_HostPathValue:
		host := x.HostPathValue

		name := hash(host.Context)

		target := &proto.FilesystemPath{}
		if host.Path.GetDir() != nil {
			target.Path = &proto.FilesystemPath_Dir{
				Dir: &proto.DirPath{
					Path: path.Join(name, host.Path.GetDir().Path),
				},
			}
		} else {
			target.Path = &proto.FilesystemPath_File{
				File: &proto.FilePath{
					Path: path.Join(name, host.Path.GetFile().Path),
				},
			}
		}

		targetPath := target.FromSlash()
		if !cmd.mounted[targetPath] {
			cmd.Mounts = append(cmd.Mounts, CommandMount{
				Source: &proto.ThunkMountSource{
					Source: &proto.ThunkMountSource_HostSource{
						HostSource: host,
					},
				},
				Target: targetPath,
			})

			cmd.mounted[targetPath] = true
		}

		return proto.NewValue(&proto.String{Inner: cmd.rel(target)})
	case *proto.Value_FsPathValue:
		embedPath := x.FsPathValue

		name := hash("embed:" + embedPath.Id)

		target := &proto.FilesystemPath{}
		if embedPath.Path.GetDir() != nil {
			target.Path = &proto.FilesystemPath_Dir{
				Dir: &proto.DirPath{
					Path: path.Join(name, embedPath.Path.GetDir().Path),
				},
			}
		} else {
			target.Path = &proto.FilesystemPath_File{
				File: &proto.FilePath{
					Path: path.Join(name, embedPath.Path.GetFile().Path),
				},
			}
		}

		targetPath := target.FromSlash()
		if !cmd.mounted[targetPath] {
			cmd.Mounts = append(cmd.Mounts, CommandMount{
				Source: &proto.ThunkMountSource{
					Source: &proto.ThunkMountSource_FsSource{
						FsSource: embedPath,
					},
				},
				Target: targetPath,
			})

			cmd.mounted[targetPath] = true
		}

		return proto.NewValue(&proto.String{Inner: cmd.rel(target)})
	case *proto.Value_SecretValue:
		secret := x.SecretValue
		return proto.NewValue(&proto.String{Inner: string(secret.Value)})
	}

	return val, nil
}

func (cmd *Command) resolveString(val *proto.Value) (string, error) {
	resolved, err := cmd.resolveValue(val)
	if err != nil {
		return "", err
	}

	switch x := resolved.GetValue().(type) {
	case *proto.Value_StringValue:
		return x.StringValue.Inner, nil
	case *proto.Value_ArrayValue:
		var str string
		for _, v := range x.ArrayValue.Values {
			s, err := cmd.resolveString(v)
			if err != nil {
				return "", err
			}
			str += s
		}

		return str, nil
	default:
		return "", fmt.Errorf("cannot stringify: %T", resolved.GetValue())
	}
}

func (cmd *Command) rel(workRelPath *proto.FilesystemPath) string {
	if cmd.Dir == nil {
		return workRelPath.FromSlash()
	}

	var cwdRelPath = workRelPath.FromSlash()
	for dir := filepath.Dir(*cmd.Dir); dir != "."; dir = filepath.Dir(dir) {
		cwdRelPath = filepath.Join("..", cwdRelPath)
	}

	if workRelPath.GetDir() != nil {
		cwdRelPath += string(os.PathSeparator)
	}

	return cwdRelPath
}
