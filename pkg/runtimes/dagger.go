//go:generate dagger client-gen -o ./dagger/api.gen.go

package runtimes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	kitdclient "github.com/moby/buildkit/client"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/tonistiigi/fsutil"
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
		Progress:  statusProxy.Writer(),
		LocalDirs: map[string]string{},
	}

	if err := runtime.collectLocalDirs(ctx, opts.LocalDirs, thunk); err != nil {
		return err
	}

	return engine.Start(ctx, opts, func(ctx engine.Context) error {
		core := dagger.New(ctx.Client)

		builder := runtime.newBuilder(core)

		ctr, err := builder.container(ctx, thunk)
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

	// XXX(vito): this is hanging for some reason
	// defer statusProxy.Wait()

	opts := &engine.Config{
		Progress:  statusProxy.Writer(),
		LocalDirs: map[string]string{},
	}

	if err := runtime.collectLocalDirs(ctx, opts.LocalDirs, thunk); err != nil {
		return err
	}

	return engine.Start(ctx, opts, func(ctx engine.Context) error {
		core := dagger.New(ctx.Client)

		builder := runtime.newBuilder(core)

		ctr, err := builder.container(ctx, thunk)
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

func (runtime *Dagger) collectLocalDirs(ctx context.Context, dest map[string]string, thunk bass.Thunk) error {
	cmd, err := NewCommand(ctx, runtime, thunk)
	if err != nil {
		return err
	}

	for _, mnt := range cmd.Mounts {
		if mnt.Source.HostPath != nil {
			dest[mnt.Source.HostPath.Hash()] = mnt.Source.HostPath.ContextDir
		} else if mnt.Source.ThunkPath != nil {
			err := runtime.collectLocalDirs(ctx, dest, mnt.Source.ThunkPath.Thunk)
			if err != nil {
				return fmt.Errorf("collect local dirs for mount %s: %w", mnt.Target, err)
			}
		}
	}

	return nil
}

func (runtime *Dagger) ExportPath(ctx context.Context, w io.Writer, tp bass.ThunkPath) error {
	dir, err := os.MkdirTemp("", "bass-dagger-export*")
	if err != nil {
		return err
	}

	defer os.RemoveAll(dir)

	statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
	defer statusProxy.Wait()

	opts := &engine.Config{
		Progress:  statusProxy.Writer(),
		LocalDirs: map[string]string{"export": dir},
	}

	if err := runtime.collectLocalDirs(ctx, opts.LocalDirs, tp.Thunk); err != nil {
		return err
	}

	walkOpts := &fsutil.WalkOpt{}
	err = engine.Start(ctx, opts, func(ctx engine.Context) error {
		core := dagger.New(ctx.Client)

		builder := runtime.newBuilder(core)

		ctr, err := builder.container(ctx, tp.Thunk)
		if err != nil {
			return err
		}

		fsp := tp.Path.FilesystemPath()
		if fsp.IsDir() {
			srcID, err := ctr.Directory(fsp.Slash()).ID(ctx)
			if err != nil {
				return fmt.Errorf("get source dir ID: %w", err)
			}

			ok, err := core.Host().Directory("export").Write(ctx, srcID)
			if err != nil {
				return fmt.Errorf("write to export dir: %w", err)
			}

			if !ok {
				return fmt.Errorf("write to export dir: not ok")
			}
		} else {
			// TODO: it'd be great if I didn't have to export the entire directory!

			walkOpts.IncludePatterns = []string{fsp.Name()}

			srcID, err := ctr.Directory(fsp.Dir().Slash()).ID(ctx)
			if err != nil {
				return fmt.Errorf("get source dir ID: %w", err)
			}

			ok, err := core.Host().Directory("export").Write(ctx, srcID)
			if err != nil {
				return fmt.Errorf("write to export dir: %w", err)
			}

			if !ok {
				return fmt.Errorf("write to export dir: not ok")
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("engine: %w", err)
	}

	return fsutil.WriteTar(ctx, fsutil.NewFS(dir, walkOpts), w)
}

func (runtime *Dagger) Prune(ctx context.Context, opts bass.PruneOpts) error {
	return errors.New("Prune: not implemented")
}

func (runtime *Dagger) Close() error {
	return nil
}

func (runtime *Dagger) newBuilder(core *dagger.Query) *daggerBuilder {
	return &daggerBuilder{
		runtime: runtime,
		core:    core,
	}
}

type daggerBuilder struct {
	runtime *Dagger
	core    *dagger.Query
}

func (b *daggerBuilder) container(ctx context.Context, thunk bass.Thunk) (*dagger.Container, error) {
	cmd, err := NewCommand(ctx, b.runtime, thunk)
	if err != nil {
		return nil, err
	}

	imageRef, baseContainer, err := b.image(ctx, thunk.Image)
	if err != nil {
		return nil, err
	}

	var root *dagger.Container
	if baseContainer != nil {
		root = baseContainer
	} else {
		root = b.core.Container().From(imageRef)
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
		mounted, err := b.mount(ctx, ctr, mount.Target, mount.Source)
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

	return ctr.Exec(dagger.ContainerExecOpts{
		Args: cmd.Args,
		Opts: dagger.ExecOpts{
			Stdin: string(cmd.Stdin),
		},
	}), nil
}

func (b *daggerBuilder) mount(ctx context.Context, ctr *dagger.Container, target string, src bass.ThunkMountSource) (*dagger.Container, error) {
	if !path.IsAbs(target) {
		target = path.Join(workDir, target)
	}

	switch {
	case src.ThunkPath != nil:
		srcCtr, err := b.container(ctx, src.ThunkPath.Thunk)
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

		cacheID, err := b.core.CacheFromTokens([]string{src.Cache.ID}).ID(ctx)
		if err != nil {
			return nil, err
		}

		return ctr.WithMountedCache(cacheID, target), nil
	case src.FSPath != nil:
		dir := b.core.Directory()

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

			dir = dir.WithNewFile(entry, dagger.DirectoryWithNewFileOpts{
				Contents: string(content),
			})

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
	case src.HostPath != nil:
		id := src.HostPath.Hash()

		hostDir := b.core.Host().Directory(dagger.HostDirectoryID(id)).Read()

		fsp := src.HostPath.Path.FilesystemPath()
		if fsp.IsDir() {
			dirID, err := hostDir.Directory(fsp.Slash()).ID(ctx)
			if err != nil {
				return nil, err
			}

			return ctr.WithMountedDirectory(target, dirID), nil
		} else {
			fileID, err := hostDir.File(fsp.Slash()).ID(ctx)
			if err != nil {
				return nil, fmt.Errorf("get host file ID: %w", err)
			}

			return ctr.WithMountedFile(target, fileID), nil
		}
	default:
		return nil, fmt.Errorf("mounting %T not implemented yet", src.ToValue())
	}
}

func (b *daggerBuilder) image(ctx context.Context, image *bass.ThunkImage) (dagger.ContainerAddress, *dagger.Container, error) {
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
		ctr, err := b.container(ctx, *image.Thunk)
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
