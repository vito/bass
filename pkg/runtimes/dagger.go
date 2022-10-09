//go:generate dagger client-gen -o ./dagger/api.gen.go

package runtimes

import (
	"context"
	"errors"
	"fmt"
	"io"

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

	// TODO: use as hostname (also Dagger should support arbitrary labels?)
	// id, err := thunk.Hash()
	// if err != nil {
	// 	return nil, err
	// }

	// TODO: TLS and service networking, but Dagger needs to figure that out
	// first

	ctr := root.
		WithMountedTemp("/tmp").
		WithMountedTemp("/dev/shm").
		WithWorkdir(workDir).
		Exec(dagger.WithContainerExecArgs(cmd.Args))

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

	return ctr, nil
}

func (runtime *Dagger) mount(ctx context.Context, core *dagger.Query, ctr *dagger.Container, target string, src bass.ThunkMountSource) (*dagger.Container, error) {
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
			return nil, fmt.Errorf("mounting files not implemented yet")

			// id, err := srcCtr.File(fsp.Slash()).ID(ctx)
			// if err != nil {
			// 	return nil, err
			// }

			// return ctr.WithMountedDirectory(target, id), nil
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
