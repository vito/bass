//go:generate dagger client-gen -o ./dagger/dagger.gen.go

package runtimes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"strings"

	"dagger.io/dagger/engine"
	"dagger.io/dagger/router"
	"dagger.io/dagger/sdk/go/dagger"
	"github.com/adrg/xdg"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/tonistiigi/fsutil"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/progrock"
)

const DaggerName = "dagger"

func init() {
	RegisterRuntime(DaggerName, NewDagger)
}

type Dagger struct {
	Platform ocispecs.Platform

	sockPath string
	stop     func()
	stopped  chan struct{}

	client *dagger.Client
}

var _ bass.Runtime = &Dagger{}

type DaggerConfig struct {
	Host string `json:"host,omitempty"`
}

func NewDagger(ctx context.Context, _ bass.RuntimePool, cfg *bass.Scope) (bass.Runtime, error) {
	var config DaggerConfig
	if cfg != nil {
		if err := cfg.Decode(&config); err != nil {
			return nil, fmt.Errorf("dagger runtime config: %w", err)
		}
	}

	engineCtx, cancel := context.WithCancel(ctx)

	runtime := &Dagger{
		sockPath: fmt.Sprintf("bass/dagger.%d.sock", os.Getpid()),
		stop:     cancel,
		stopped:  make(chan struct{}),
	}

	if config.Host == "" {
		if err := os.RemoveAll(runtime.sockPath); err != nil {
			return nil, err
		}

		sockPath, err := xdg.StateFile(runtime.sockPath)
		if err != nil {
			return nil, err
		}

		config.Host = "unix://" + sockPath

		l, err := net.Listen("unix", sockPath)
		if err != nil {
			return nil, fmt.Errorf("listen for dagger conns: %w", err)
		}

		statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))

		go engine.Start(engineCtx, &engine.Config{
			RawBuildkitStatus: statusProxy.Writer(),
		}, func(ctx context.Context, r *router.Router) error {
			go func() {
				for {
					conn, err := l.Accept()
					if err != nil {
						break
					}

					r.ServeConn(conn)
				}
			}()

			<-ctx.Done()

			statusProxy.Wait()
			close(runtime.stopped)
			return nil
		})
	}

	os.Setenv("DAGGER_HOST", config.Host)

	client, err := dagger.Connect(ctx)
	if err != nil {
		return nil, err
	}

	runtime.client = client

	return runtime, nil
}

func (runtime *Dagger) Resolve(ctx context.Context, imageRef bass.ImageRef) (bass.ImageRef, error) {
	// TODO
	return imageRef, nil
}

func (runtime *Dagger) Run(ctx context.Context, thunk bass.Thunk) error {
	ctr, err := runtime.container(ctx, thunk)
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
}

func (runtime *Dagger) Start(ctx context.Context, thunk bass.Thunk) (StartResult, error) {
	return StartResult{}, errors.New("Start: not implemented")
}

func (runtime *Dagger) Read(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	ctr, err := runtime.container(ctx, thunk)
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
}

func (runtime *Dagger) Export(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	return errors.New("Export: not implemented")
}

func (runtime *Dagger) ExportPath(ctx context.Context, w io.Writer, tp bass.ThunkPath) error {
	dir, err := os.MkdirTemp("", "bass-dagger-export*")
	if err != nil {
		return err
	}

	defer os.RemoveAll(dir)

	ctr, err := runtime.container(ctx, tp.Thunk)
	if err != nil {
		return err
	}

	walkOpts := &fsutil.WalkOpt{}

	fsp := tp.Path.FilesystemPath()
	if fsp.IsDir() {
		ok, err := ctr.Directory(fsp.Slash()).Export(ctx, dir)
		if err != nil {
			return fmt.Errorf("write to export dir: %w", err)
		}

		if !ok {
			return fmt.Errorf("write to export dir: not ok")
		}
	} else {
		walkOpts.IncludePatterns = []string{fsp.Name()}

		// TODO: it'd be great if I didn't have to export the entire directory!
		ok, err := ctr.Directory(fsp.Dir().Slash()).Export(ctx, dir)
		if err != nil {
			return fmt.Errorf("write to export dir: %w", err)
		}

		if !ok {
			return fmt.Errorf("write to export dir: not ok")
		}
	}

	return fsutil.WriteTar(ctx, fsutil.NewFS(dir, walkOpts), w)
}

func (runtime *Dagger) Prune(ctx context.Context, opts bass.PruneOpts) error {
	return errors.New("Prune: not implemented")
}

func (runtime *Dagger) Close() error {
	err := runtime.client.Close()
	if err != nil {
		return fmt.Errorf("close client: %w", err)
	}

	runtime.stop()

	<-runtime.stopped

	if err := os.RemoveAll(runtime.sockPath); err != nil {
		return err
	}

	return nil
}

func (runtime *Dagger) container(ctx context.Context, thunk bass.Thunk) (*dagger.Container, error) {
	cmd, err := NewCommand(ctx, runtime, thunk)
	if err != nil {
		return nil, err
	}

	imageRef, baseContainer, err := runtime.image(ctx, thunk.Image)
	if err != nil {
		return nil, err
	}

	var root *dagger.Container
	if baseContainer != nil {
		root = baseContainer
	} else {
		root = runtime.client.Container().From(imageRef)
	}

	// TODO: TLS and service networking, but Dagger needs to figure that out
	// first

	ctr := root.
		WithMountedTemp("/tmp").
		WithMountedTemp("/dev/shm").
		WithEntrypoint(nil).
		WithWorkdir(workDir)

	// TODO: set hostname instead once Dagger supports it
	id, err := thunk.Hash()
	if err != nil {
		return nil, err
	}
	ctr = ctr.WithEnvVariable("THUNK", id)

	// TODO: insecure
	// if thunk.Insecure {
	// 	needsInsecure = true

	// 	runOpt = append(runOpt,
	// 		llb.WithCgroupParent(id),
	// 		llb.Security(llb.SecurityModeInsecure))
	// }

	for _, mount := range cmd.Mounts {
		mounted, err := runtime.mount(ctx, ctr, mount.Target, mount.Source)
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
		ctr = ctr.WithEnvVariable(name, val)
	}

	return ctr.Exec(dagger.ContainerExecOpts{
		Args:  cmd.Args,
		Stdin: string(cmd.Stdin),
	}), nil
}

func (runtime *Dagger) mount(ctx context.Context, ctr *dagger.Container, target string, src bass.ThunkMountSource) (*dagger.Container, error) {
	if !path.IsAbs(target) {
		target = path.Join(workDir, target)
	}

	switch {
	case src.ThunkPath != nil:
		srcCtr, err := runtime.container(ctx, src.ThunkPath.Thunk)
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

		cacheID, err := runtime.client.CacheVolume(src.Cache.ID).ID(ctx)
		if err != nil {
			return nil, err
		}

		return ctr.WithMountedCache(cacheID, target), nil
	case src.FSPath != nil:
		dir := runtime.client.Directory()

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
		fsp := src.HostPath.Path.FilesystemPath()

		if fsp.IsDir() {
			hostDir, err := runtime.client.Host().Directory(fsp.FromSlash()).ID(ctx)
			if err != nil {
				return nil, fmt.Errorf("get host dir: %w", err)
			}

			return ctr.WithMountedDirectory(target, hostDir), nil
		} else {
			fileID, err := runtime.client.Host().Directory(fsp.Dir().FromSlash()).File(fsp.Name()).ID(ctx)
			if err != nil {
				return nil, fmt.Errorf("get host file: %w", err)
			}

			return ctr.WithMountedFile(target, fileID), nil
		}
	default:
		return nil, fmt.Errorf("mounting %T not implemented yet", src.ToValue())
	}
}

func (runtime *Dagger) image(ctx context.Context, image *bass.ThunkImage) (string, *dagger.Container, error) {
	if image == nil {
		return "", nil, nil
	}

	if image.Ref != nil {
		ref, err := image.Ref.Ref()
		if err != nil {
			return "", nil, err
		}

		return ref, nil, nil
	}

	if image.Thunk != nil {
		ctr, err := runtime.container(ctx, *image.Thunk)
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
