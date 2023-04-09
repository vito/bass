package runtimes

import (
	"bytes"
	"context"
	"fmt"
	"net"
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

	// these don't need to be marshaled, since they're part of the container
	// setup and not passed to the shim
	Mounts    []CommandMount     `json:"-"`
	Services  []bass.Thunk       `json:"-"`
	SecretEnv []CommandSecretEnv `json:"-"`

	mounted map[string]bool
	starter Starter
}

// CommandMount configures a thunk path to mount to the command's container.
type CommandMount struct {
	Source bass.ThunkMountSource
	Target string
}

type CommandHost struct {
	Host   string
	Target net.IP
}

type CommandSecretEnv struct {
	Name   string
	Secret bass.Secret
}

type Starter interface {
	// Start starts the thunk and waits for its ports to be ready.
	Start(context.Context, bass.Thunk) (StartResult, error)
}

type StartResult struct {
	// A mapping from each port to its address info (host, port, etc.)
	Ports PortInfos
}

type PortInfos map[string]*bass.Scope

// Resolve traverses the Thunk, resolving logical path values to their
// concrete paths in the container, and collecting the requisite mount points
// along the way.
func NewCommand(ctx context.Context, starter Starter, thunk bass.Thunk) (Command, error) {
	cmd := &Command{
		mounted: map[string]bool{},
		starter: starter,
	}

	if thunk.Dir != nil {
		var cwd string
		err := cmd.resolveValue(ctx, thunk.Dir.ToValue(), &cwd)
		if err != nil {
			return Command{}, fmt.Errorf("resolve wd: %w", err)
		}

		cmd.Dir = &cwd
	}

	var err error
	cmd.Args, err = cmd.resolveArgs(ctx, thunk.Args)
	if err != nil {
		return Command{}, fmt.Errorf("resolve args: %w", err)
	}

	if thunk.Env != nil {
		err := thunk.Env.Each(func(name bass.Symbol, v bass.Value) error {
			var null bass.Null
			if v.Decode(&null) == nil {
				// env tombstone; skip it
				return nil
			}

			var secret bass.Secret
			if v.Decode(&secret) == nil {
				cmd.SecretEnv = append(cmd.SecretEnv, CommandSecretEnv{
					Name:   name.JSONKey(),
					Secret: secret,
				})
				return nil
			}

			val, err := cmd.resolveStr(ctx, v)
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
		stdin, err := cmd.resolveValues(ctx, thunk.Stdin)
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

	// empty out fields only needed during creation so we can test with equality
	cmd.mounted = nil
	cmd.starter = nil

	return *cmd, nil
}

func (cmd Command) Equal(other Command) bool {
	return cmp.Equal(cmd.Args, other.Args) &&
		cmp.Equal(cmd.Stdin, other.Stdin) &&
		cmp.Equal(cmd.Env, other.Env) &&
		cmp.Equal(cmd.Dir, other.Dir) &&
		cmp.Equal(cmd.Mounts, other.Mounts)
}

func (cmd *Command) resolveStr(ctx context.Context, val bass.Value) (string, error) {
	var str string

	var concat bass.List
	if err := val.Decode(&concat); err == nil {
		err := cmd.resolveArg(ctx, concat, &str)
		if err != nil {
			return "", fmt.Errorf("concat: %w", err)
		}
	} else if err := cmd.resolveValue(ctx, val, &str); err != nil {
		return "", err
	}

	return str, nil
}

func (cmd *Command) resolveArgs(ctx context.Context, list []bass.Value) ([]string, error) {
	var args []string
	for i, v := range list {
		arg, err := cmd.resolveStr(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("arg %d: %w", i, err)
		}

		args = append(args, arg)
	}

	return args, nil
}

func (cmd *Command) resolveValues(ctx context.Context, list []bass.Value) ([]bass.Value, error) {
	var vals []bass.Value
	for _, v := range list {
		resolved, err := bass.Resolve(v, func(v2 bass.Value) (bass.Value, error) {
			var val bass.Value
			err := cmd.resolveValue(ctx, v2, &val)
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

func (cmd *Command) resolveValue(ctx context.Context, val bass.Value, dest any) error {
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
		name, err := artifact.Thunk.Hash()
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
			Path: host.Hash(),
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

	var cache bass.CachePath
	if err := val.Decode(&cache); err == nil {
		target, err := bass.DirPath{
			Path: cache.Hash(),
		}.Extend(cache.Path.FilesystemPath())
		if err != nil {
			return err
		}

		fsp := target.(bass.FilesystemPath)

		targetPath := fsp.FromSlash()
		if !cmd.mounted[targetPath] {
			cmd.Mounts = append(cmd.Mounts, CommandMount{
				Source: bass.ThunkMountSource{
					Cache: &cache,
				},
				Target: targetPath,
			})

			cmd.mounted[targetPath] = true
		}

		return bass.String(cmd.rel(fsp)).Decode(dest)
	}

	var embedPath *bass.FSPath
	if err := val.Decode(&embedPath); err == nil {
		hash, err := embedPath.Hash()
		if err != nil {
			return err
		}

		target, err := bass.DirPath{
			Path: hash,
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

	var addr bass.ThunkAddr
	if err := val.Decode(&addr); err == nil {
		result, err := cmd.starter.Start(ctx, addr.Thunk)
		if err != nil {
			return fmt.Errorf("start %s: %w", addr.Thunk, err)
		}

		info, found := result.Ports[addr.Port]
		if !found {
			return fmt.Errorf("no info for port '%s': %+v", addr.Port, result.Ports)
		}

		str, err := addr.Render(info)
		if err != nil {
			return err
		}

		cmd.Services = append(cmd.Services, addr.Thunk)

		return bass.String(str).Decode(dest)
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

func (cmd *Command) resolveArg(ctx context.Context, vals bass.List, dest any) error {
	var res string
	err := bass.Each(vals, func(v bass.Value) error {
		var val string
		err := cmd.resolveValue(ctx, v, &val)
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
