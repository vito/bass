//go:generate dagger client-gen -o ./dagger/dagger.gen.go

package runtimes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"dagger.io/dagger"
	"github.com/docker/distribution/reference"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/tonistiigi/fsutil"
	"github.com/vito/bass/pkg/bass"
)

const DaggerName = "dagger"

func init() {
	RegisterRuntime(DaggerName, NewDagger)
}

type Dagger struct {
	Platform ocispecs.Platform

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

	client, err := dagger.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return &Dagger{
		client: client,
	}, nil
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
	result := StartResult{
		Ports: PortInfos{},
	}

	host := thunk.Name()
	for _, port := range thunk.Ports {
		result.Ports[port.Name] = bass.Bindings{
			"host": bass.String(host),
			"port": bass.Int(port.Port),
		}.Scope()
	}

	return result, nil
}

func (runtime *Dagger) Read(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	ctr, err := runtime.container(ctx, thunk)
	if err != nil {
		return err
	}

	stdout, err := ctr.Stdout(ctx)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(w, stdout)
	if err != nil {
		return err
	}

	return nil
}

func (runtime *Dagger) Publish(ctx context.Context, ref bass.ImageRef, thunk bass.Thunk) (bass.ImageRef, error) {
	ctr, err := runtime.container(ctx, thunk)
	if err != nil {
		return ref, err
	}

	addr, err := ref.Ref()
	if err != nil {
		return ref, err
	}

	fqref, err := ctr.Publish(ctx, addr)
	if err != nil {
		return ref, err
	}

	fq, err := reference.ParseNamed(fqref)
	if err != nil {
		return ref, err
	}

	canon, ok := fq.(reference.Canonical)
	if !ok {
		return ref, fmt.Errorf("Dagger did not return a canonical reference: %T: %s", fq, fqref)
	}

	ref.Digest = canon.Digest().String()

	return ref, nil
}

func (runtime *Dagger) Export(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	ctr, err := runtime.container(ctx, thunk)
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "bass-dagger-export*")
	if err != nil {
		return err
	}

	defer os.RemoveAll(dir)

	image := filepath.Join(dir, "image.tar")
	ok, err := ctr.Export(ctx, image)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("write to export dir: not ok")
	}

	f, err := os.Open(image)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = io.Copy(w, f)
	return err
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

	fsp := tp.Path.FilesystemPath()

	var ok bool
	if fsp.IsDir() {
		ok, err = ctr.Directory(fsp.Slash()).Export(ctx, dir)
	} else {
		ok, err = ctr.File(fsp.Slash()).Export(ctx, filepath.Join(dir, fsp.Name()))
	}
	if err != nil {
		return fmt.Errorf("export file: %w", err)
	}

	if !ok {
		return fmt.Errorf("export failed: not ok")
	}

	return fsutil.WriteTar(ctx, fsutil.NewFS(dir, &fsutil.WalkOpt{}), w)
}

func (runtime *Dagger) Prune(ctx context.Context, opts bass.PruneOpts) error {
	return errors.New("Prune: not implemented")
}

func (runtime *Dagger) Close() error {
	return runtime.client.Close()
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

	ctr := root.
		WithMountedTemp("/tmp").
		WithMountedTemp("/dev/shm").
		WithWorkdir(workDir)

	id, err := thunk.Hash()
	if err != nil {
		return nil, err
	}

	// NB: mount the thunk hash cache-buster to match Bass behavior of busting
	// cache when thunk labels change
	ctr = ctr.WithMountedDirectory(
		"/tmp/.thunk",
		runtime.client.Directory().WithNewFile("name", id),
	)

	if thunk.Labels != nil {
		thunk.Labels.Each(func(k bass.Symbol, v bass.Value) error {
			var s string
			if err := v.Decode(&s); err != nil {
				s = v.String()
			}

			ctr = ctr.WithLabel(k.String(), s)
			return nil
		})
	}

	for _, port := range thunk.Ports {
		ctr = ctr.WithExposedPort(port.Port, dagger.ContainerWithExposedPortOpts{
			Description: port.Name,
		})
	}

	// TODO: TLS

	for _, svc := range cmd.Services {
		svcCtr, err := runtime.container(ctx, svc)
		if err != nil {
			return nil, err
		}

		ctr = ctr.WithServiceBinding(svc.Name(), svcCtr)
	}

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

	for _, env := range cmd.SecretEnv {
		secret := runtime.client.SetSecret(
			env.Secret.Name,
			string(env.Secret.Reveal()),
		)
		ctr = ctr.WithSecretVariable(env.Name, secret)
	}

	if cmd.Args != nil {
		ctr = ctr.WithExec(cmd.Args, dagger.ContainerWithExecOpts{
			Stdin:                    string(cmd.Stdin),
			InsecureRootCapabilities: thunk.Insecure,
		})
	}

	if thunk.Entrypoint != nil {
		ctr = ctr.WithEntrypoint(thunk.Entrypoint)
	}

	if thunk.DefaultArgs != nil {
		ctr = ctr.WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
			Args: thunk.DefaultArgs,
		})
	}

	return ctr, nil
}

var epoch = time.Date(1985, 10, 26, 8, 15, 0, 0, time.UTC)

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
			return ctr.WithMountedDirectory(
				target,
				srcCtr.Directory(fsp.Slash()).WithTimestamps(int(epoch.Unix())),
			), nil
		} else {
			return ctr.WithMountedFile(
				target,
				srcCtr.File(fsp.Slash()).WithTimestamps(int(epoch.Unix())),
			), nil
		}
	case src.Cache != nil:
		fsp := src.Cache.Path.FilesystemPath()
		if fsp.Slash() != "./" {
			return nil, fmt.Errorf("mounting subpaths of cache not implemented yet: %s", fsp.Slash())
		}

		return ctr.WithMountedCache(
			target,
			runtime.client.CacheVolume(src.Cache.ID),
			dagger.ContainerWithMountedCacheOpts{
				Sharing: dagger.Locked,
			},
		), nil
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

			dir = dir.WithNewFile(entry, string(content))

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", root, err)
		}

		fsp := src.FSPath.Path.FilesystemPath()
		if fsp.IsDir() {
			return ctr.WithMountedDirectory(target, dir.Directory(fsp.Slash())), nil
		} else {
			return ctr.WithMountedFile(target, dir.File(fsp.Slash())), nil
		}
	case src.HostPath != nil:
		dir := runtime.client.Host().Directory(src.HostPath.ContextDir)
		fsp := src.HostPath.Path.FilesystemPath()

		if fsp.IsDir() {
			return ctr.WithMountedDirectory(target, dir.Directory(fsp.FromSlash())), nil
		} else {
			return ctr.WithMountedFile(target, dir.File(fsp.FromSlash())), nil
		}
	case src.Secret != nil:
		secret := runtime.client.SetSecret(src.Secret.Name, string(src.Secret.Reveal()))
		return ctr.WithMountedSecret(target, secret), nil
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

	return "", nil, fmt.Errorf("unsupported image type: %s", image.ToValue())
}
