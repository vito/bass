//go:generate dagger client-gen -o ./dagger/api.gen.go

package runtimes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	kitdclient "github.com/moby/buildkit/client"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/runtimes/dagger"
	"github.com/vito/progrock"
	"go.dagger.io/dagger/engine"
)

const DaggerName = "dagger"

func init() {
	RegisterRuntime(DaggerName, NewDagger)
}

type Dagger struct {
	Config   BuildkitConfig
	Client   *kitdclient.Client
	Platform ocispecs.Platform
}

var _ bass.Runtime = &Dagger{}

func NewDagger(ctx context.Context, _ bass.RuntimePool, cfg *bass.Scope) (bass.Runtime, error) {
	r := &Dagger{}

	if cfg != nil {
		if err := cfg.Decode(&r.Config); err != nil {
			return nil, fmt.Errorf("dagger runtime config: %w", err)
		}
	}

	return r, nil
}

func (runtime *Dagger) Resolve(ctx context.Context, imageRef bass.ImageRef) (bass.ImageRef, error) {
	// TODO
	return imageRef, nil
}

func (runtime *Dagger) Run(ctx context.Context, thunk bass.Thunk) error {
	statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
	defer statusProxy.Wait()

	opts := &engine.Config{
		Progress: statusProxy.Writer(),
	}

	return engine.Start(ctx, opts, func(ctx engine.Context) error {
		core := dagger.New(ctx.Client)

		ctr, err := runtime.container(ctx, core, thunk)
		if err != nil {
			return err
		}

		status, err := ctr.ExitCode(ctx)
		if err != nil {
			return err
		}

		if status != 0 {
			return fmt.Errorf("exit status %d", status)
		}

		return nil
	})
}

func (runtime *Dagger) Start(ctx context.Context, thunk bass.Thunk) (StartResult, error) {
	return StartResult{}, errors.New("Start: not implemented")
}

func (runtime *Dagger) Read(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
	defer statusProxy.Wait()

	opts := &engine.Config{
		Progress: statusProxy.Writer(),
	}

	return engine.Start(ctx, opts, func(ctx engine.Context) error {
		core := dagger.New(ctx.Client)

		ctr, err := runtime.container(ctx, core, thunk)
		if err != nil {
			return err
		}

		stdout, err := ctr.Stdout().Contents(ctx)
		if err != nil {
			return err
		}

		_, err = fmt.Fprint(w, stdout)
		if err != nil {
			return err
		}

		return nil
	})
}

func (runtime *Dagger) Export(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	return errors.New("Export: not implemented")
}

func (runtime *Dagger) ExportPath(ctx context.Context, w io.Writer, tp bass.ThunkPath) error {
	return errors.New("ExportPath: not implemented")
}

func (runtime *Dagger) Prune(ctx context.Context, opts bass.PruneOpts) error {
	return errors.New("Prune: not implemented")
}

func (runtime *Dagger) Close() error {
	return nil
}

func (runtime *Dagger) container(ctx context.Context, core *dagger.Query, thunk bass.Thunk) (*dagger.Container, error) {
	cmd, err := NewCommand(ctx, runtime, thunk)
	if err != nil {
		return nil, err
	}

	imageRef, baseContainer, err := runtime.image(ctx, core, thunk.Image)
	if err != nil {
		return nil, err
	}

	var root *dagger.Container
	if baseContainer != nil {
		root = baseContainer
	} else {
		root = core.Container().From(imageRef)
	}

	// TODO: TLS and service networking, but Dagger needs to figure that out
	// first

	ctr := root.
		WithMountedTemp("/tmp").
		WithMountedTemp("/dev/shm").
		WithWorkdir(workDir)

	// TODO: set hostname instead once Dagger supports it
	id, err := thunk.Hash()
	if err != nil {
		return nil, err
	}
	ctr = ctr.WithVariable("THUNK", id)

	// TODO: insecure
	// if thunk.Insecure {
	// 	needsInsecure = true

	// 	runOpt = append(runOpt,
	// 		llb.WithCgroupParent(id),
	// 		llb.Security(llb.SecurityModeInsecure))
	// }

	for _, mount := range cmd.Mounts {
		mounted, err := runtime.mount(ctx, core, ctr, mount.Target, mount.Source)
		if err != nil {
			return nil, err
		}

		ctr = mounted
	}

	// TODO: cache disabling in Dagger?
	// if b.runtime.Config.DisableCache {
	// 	runOpt = append(runOpt, llb.IgnoreCache)
	// }

	// runOpt = append(runOpt, extraOpts...)

	if cmd.Dir != nil {
		ctr = ctr.WithWorkdir(*cmd.Dir)
	}

	for _, env := range cmd.Env {
		name, val, ok := strings.Cut(env, "=")
		_ = ok // doesnt matter
		ctr = ctr.WithVariable(name, val)
	}

	execOpts := []dagger.ContainerExecOption{
		dagger.WithContainerExecArgs(cmd.Args),
	}

	if cmd.Stdin != nil {
		execOpts = append(execOpts, dagger.WithContainerExecOpts(dagger.ExecOpts{
			Stdin: string(cmd.Stdin),
		}))
	}

	return ctr.Exec(execOpts...), nil
}

func (runtime *Dagger) mount(ctx context.Context, core *dagger.Query, ctr *dagger.Container, target string, src bass.ThunkMountSource) (*dagger.Container, error) {
	if !path.IsAbs(target) {
		target = path.Join(workDir, target)
	}

	switch {
	case src.ThunkPath != nil:
		srcCtr, err := runtime.container(ctx, core, src.ThunkPath.Thunk)
		if err != nil {
			return nil, err
		}

		fsp := src.ThunkPath.Path.FilesystemPath()
		if fsp.IsDir() {
			id, err := srcCtr.Directory(fsp.Slash()).ID(ctx)
			if err != nil {
				return nil, err
			}

			return ctr.WithMountedDirectory(target, id), nil
		} else {
			id, err := srcCtr.File(fsp.Slash()).ID(ctx)
			if err != nil {
				return nil, err
			}

			return ctr.WithMountedFile(target, id), nil
		}
	case src.Cache != nil:
		fsp := src.Cache.Path.FilesystemPath()
		if fsp.Slash() != "./" {
			return nil, fmt.Errorf("mounting subpaths of cache not implemented yet: %s", fsp.Slash())
		}

		cacheID, err := core.CacheFromTokens([]string{src.Cache.ID}).ID(ctx)
		if err != nil {
			return nil, err
		}

		return ctr.WithMountedCache(cacheID, target), nil
	case src.FSPath != nil:
		dir := core.Directory()

		root := path.Clean(src.FSPath.Path.Slash())
		err := fs.WalkDir(src.FSPath.FS, ".", func(entry string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			content, err := fs.ReadFile(src.FSPath.FS, entry)
			if err != nil {
				return fmt.Errorf("read fs %s: %w", entry, err)
			}

			dir = dir.WithNewFile(entry, dagger.WithDirectoryWithNewFileContents(string(content)))

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", root, err)
		}

		fsp := src.FSPath.Path.FilesystemPath()
		if fsp.IsDir() {
			dirID, err := dir.Directory(fsp.Slash()).ID(ctx)
			if err != nil {
				return nil, err
			}

			return ctr.WithMountedDirectory(target, dirID), nil
		} else {
			fileID, err := dir.File(fsp.Slash()).ID(ctx)
			if err != nil {
				return nil, err
			}

			return ctr.WithMountedFile(target, fileID), nil
		}
	default:
		return nil, fmt.Errorf("mounting %T not implemented yet", src.ToValue())
	}
}

func (runtime *Dagger) image(ctx context.Context, core *dagger.Query, image *bass.ThunkImage) (dagger.ContainerAddress, *dagger.Container, error) {
	if image == nil {
		return "", nil, nil
	}

	if image.Ref != nil {
		ref, err := image.Ref.Ref()
		if err != nil {
			return "", nil, err
		}

		return dagger.ContainerAddress(ref), nil, nil
	}

	if image.Thunk != nil {
		ctr, err := runtime.container(ctx, core, *image.Thunk)
		if err != nil {
			return "", nil, fmt.Errorf("image thunk llb: %w", err)
		}

		return "", ctr, nil
	}

	if image.Archive != nil {
		return "", nil, fmt.Errorf("image from archive unsupported")
	}

	return "", nil, fmt.Errorf("unsupported image type: %+v", image)
}
