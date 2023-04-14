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

func (runtime *Dagger) Resolve(ctx context.Context, imageRef bass.ImageRef) (bass.Thunk, error) {
	ref, err := imageRef.Ref()
	if err != nil {
		return bass.Thunk{}, err
	}

	fqref, err := runtime.client.Container().From(ref).ImageRef(ctx)
	if err != nil {
		return bass.Thunk{}, err
	}

	fq, err := reference.ParseNamed(fqref)
	if err != nil {
		return bass.Thunk{}, err
	}

	canon, ok := fq.(reference.Canonical)
	if !ok {
		return bass.Thunk{}, fmt.Errorf("Dagger did not return a canonical reference: %T: %s", fq, fqref)
	}

	imageRef.Digest = canon.Digest().String()

	return imageRef.Thunk(), nil
}

func (runtime *Dagger) Run(ctx context.Context, thunk bass.Thunk) error {
	ctr, err := runtime.container(ctx, runtime.client, thunk, true)
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
	ctr, err := runtime.container(ctx, runtime.client, thunk, true)
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
	ctr, err := runtime.container(ctx, runtime.client, thunk, false)
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
	ctr, err := runtime.container(ctx, runtime.client, thunk, false)
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

	ctr, err := runtime.container(ctx, runtime.client, tp.Thunk, true)
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

func (runtime *Dagger) container(ctx context.Context, client *dagger.Client, thunk bass.Thunk, forceExec bool) (*dagger.Container, error) {
	client = client.Pipeline(fmt.Sprintf("thunk %s", thunk.Name()))

	cmd, err := NewCommand(ctx, runtime, thunk)
	if err != nil {
		return nil, err
	}

	imageRef, baseContainer, err := runtime.image(ctx, runtime.client, thunk.Image)
	if err != nil {
		return nil, err
	}

	var root *dagger.Container
	if baseContainer != nil {
		root = baseContainer
	} else {
		root = client.Container().From(imageRef)
	}

	ctr := root.
		WithMountedTemp("/tmp").
		WithMountedTemp("/dev/shm").
		WithWorkdir(workDir)

	id, err := thunk.Hash()
	if err != nil {
		return nil, err
	}

	// NB: set thunk hash as a cache-buster to match Bass behavior of busting
	// cache when thunk labels change
	ctr = ctr.WithEnvVariable("_BASS_THUNK", id)

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
		svcCtr, err := runtime.container(ctx, client, svc, true)
		if err != nil {
			return nil, err
		}

		ctr = ctr.WithServiceBinding(svc.Name(), svcCtr)
	}

	for _, mount := range cmd.Mounts {
		mounted, err := runtime.mount(ctx, client, ctr, mount.Target, mount.Source)
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
		secret := client.SetSecret(
			env.Secret.Name,
			string(env.Secret.Reveal()),
		)
		ctr = ctr.WithSecretVariable(env.Name, secret)
	}

	if len(cmd.Args) > 0 {
		var removedEntrypoint bool
		var prevEntrypoint []string
		if !thunk.UseEntrypoint {
			prevEntrypoint, err = ctr.Entrypoint(ctx)
			if err != nil {
				return nil, err
			}

			if len(prevEntrypoint) > 0 {
				ctr = ctr.WithEntrypoint(nil)
				removedEntrypoint = true
			}
		}

		ctr = ctr.WithExec(cmd.Args, dagger.ContainerWithExecOpts{
			Stdin:                    string(cmd.Stdin),
			InsecureRootCapabilities: thunk.Insecure,
		})

		if removedEntrypoint {
			// restore previous entrypoint
			ctr = ctr.WithEntrypoint(prevEntrypoint)
		}
	} else if forceExec {
		ctr = ctr.WithExec(append(thunk.Entrypoint, thunk.DefaultArgs...))
	}

	if len(thunk.Entrypoint) > 0 || thunk.ClearEntrypoint {
		ctr = ctr.WithEntrypoint(thunk.Entrypoint)
	}

	if len(thunk.DefaultArgs) > 0 || thunk.ClearDefaultArgs {
		ctr = ctr.WithDefaultArgs(dagger.ContainerWithDefaultArgsOpts{
			Args: thunk.DefaultArgs,
		})
	}

	return ctr, nil
}

var epoch = time.Date(1985, 10, 26, 8, 15, 0, 0, time.UTC)

func (runtime *Dagger) mount(ctx context.Context, client *dagger.Client, ctr *dagger.Container, target string, src bass.ThunkMountSource) (*dagger.Container, error) {
	client = client.Pipeline(fmt.Sprintf("mount %s", src.ToValue()))

	if !path.IsAbs(target) {
		target = path.Join(workDir, target)
	}

	switch {
	case src.ThunkPath != nil:
		srcCtr, err := runtime.container(ctx, client, src.ThunkPath.Thunk, true)
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

		var mode dagger.CacheSharingMode
		switch src.Cache.ConcurrencyMode {
		case bass.ConcurrencyModeShared:
			mode = dagger.Shared
		case bass.ConcurrencyModePrivate:
			mode = dagger.Private
		case bass.ConcurrencyModeLocked:
			mode = dagger.Locked
		}

		return ctr.WithMountedCache(
			target,
			client.CacheVolume(src.Cache.ID),
			dagger.ContainerWithMountedCacheOpts{
				Sharing: mode,
			},
		), nil
	case src.FSPath != nil:
		dir := client.Directory()

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
		dir := client.Host().Directory(src.HostPath.ContextDir)
		fsp := src.HostPath.Path.FilesystemPath()

		if fsp.IsDir() {
			return ctr.WithMountedDirectory(target, dir.Directory(fsp.FromSlash())), nil
		} else {
			return ctr.WithMountedFile(target, dir.File(fsp.FromSlash())), nil
		}
	case src.Secret != nil:
		secret := client.SetSecret(src.Secret.Name, string(src.Secret.Reveal()))
		return ctr.WithMountedSecret(target, secret), nil
	default:
		return nil, fmt.Errorf("mounting %T not implemented yet", src.ToValue())
	}
}

func (runtime *Dagger) image(ctx context.Context, client *dagger.Client, image *bass.ThunkImage) (string, *dagger.Container, error) {
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
		ctr, err := runtime.container(ctx, client, *image.Thunk, false)
		if err != nil {
			return "", nil, fmt.Errorf("image thunk: %w", err)
		}

		return "", ctr, nil
	}

	if image.Archive != nil {
		file, err := runtime.inputFile(ctx, client, image.Archive.File)
		if err != nil {
			return "", nil, fmt.Errorf("image thunk: %w", err)
		}

		name := image.Archive.File.ToValue().String()
		if image.Archive.Tag != "" {
			name += ":" + image.Archive.Tag
		}

		client = client.Pipeline(fmt.Sprintf("import %s [platform=%s]", name, image.Archive.Platform))

		ctr := client.Container(dagger.ContainerOpts{
			Platform: dagger.Platform(image.Archive.Platform.String()),
		}).Import(file)

		return "", ctr, nil
	}

	return "", nil, fmt.Errorf("unsupported image type: %s", image.ToValue())
}

func (runtime *Dagger) inputFile(ctx context.Context, client *dagger.Client, input bass.ImageBuildInput) (*dagger.File, error) {
	switch {
	case input.Thunk != nil:
		srcCtr, err := runtime.container(ctx, client, input.Thunk.Thunk, true) // TODO: or false?
		if err != nil {
			return nil, fmt.Errorf("image thunk: %w", err)
		}

		return srcCtr.File(input.Thunk.Path.Slash()), nil
	case input.Host != nil:
		dir := client.Host().Directory(input.Host.ContextDir)
		fsp := input.Host.Path.FilesystemPath()
		return dir.File(fsp.FromSlash()), nil
	case input.FS != nil:
		dir := client.Directory()

		root := path.Clean(input.FS.Path.Slash())
		err := fs.WalkDir(input.FS.FS, ".", func(entry string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			content, err := fs.ReadFile(input.FS.FS, entry)
			if err != nil {
				return fmt.Errorf("read fs %s: %w", entry, err)
			}

			dir = dir.WithNewFile(entry, string(content))

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", root, err)
		}

		fsp := input.FS.Path.FilesystemPath()
		return dir.File(fsp.Slash()), nil
	default:
		return nil, fmt.Errorf("unknown input type: %T", input.ToValue())
	}
}
