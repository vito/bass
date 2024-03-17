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
	"dagger.io/dagger/dag"
	"github.com/docker/distribution/reference"
	"github.com/tonistiigi/fsutil"
	"github.com/vito/bass/pkg/bass"
)

const DaggerName = "dagger"

func init() {
	RegisterRuntime(DaggerName, ConfigureDagger)
}

type Dagger struct {
}

var _ bass.Runtime = &Dagger{}

type DaggerConfig struct {
	Host string `json:"host,omitempty"`
}

func ConfigureDagger(ctx context.Context, _ bass.RuntimePool, cfg *bass.Scope) (bass.Runtime, error) {
	var config DaggerConfig
	if cfg != nil {
		if err := cfg.Decode(&config); err != nil {
			return nil, fmt.Errorf("dagger runtime config: %w", err)
		}
	}

	return NewDagger(), nil
}

func NewDagger() *Dagger {
	return &Dagger{}
}

func (runtime *Dagger) Resolve(ctx context.Context, imageRef bass.ImageRef) (bass.Thunk, error) {
	ref, err := imageRef.Ref()
	if err != nil {
		return bass.Thunk{}, err
	}

	fqref, err := dag.Container().From(ref).ImageRef(ctx)
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
	ctr, err := runtime.Container(ctx, thunk, true)
	if err != nil {
		return err
	}

	_, err = ctr.Sync(ctx)
	return err
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
	ctr, err := runtime.Container(ctx, thunk, true)
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
	ctr, err := runtime.Container(ctx, thunk, false)
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
	ctr, err := runtime.Container(ctx, thunk, false)
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

	ctr, err := runtime.Container(ctx, tp.Thunk, true)
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
	return nil
}

func (runtime *Dagger) Container(ctx context.Context, thunk bass.Thunk, forceExec bool) (*dagger.Container, error) {
	cmd, err := NewCommand(ctx, runtime, thunk)
	if err != nil {
		return nil, err
	}

	ctr, err := runtime.image(ctx, thunk.Image)
	if err != nil {
		return nil, err
	}

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
		svcCtr, err := runtime.Container(ctx, svc, true)
		if err != nil {
			return nil, err
		}

		ctr = ctr.WithServiceBinding(svc.Name(), svcCtr.AsService())
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
		secret := dag.SetSecret(
			env.Secret.Name,
			string(env.Secret.Reveal()),
		)
		ctr = ctr.WithSecretVariable(env.Name, secret)
	}

	if len(cmd.Args) > 0 {
		ctr = ctr.WithExec(cmd.Args, dagger.ContainerWithExecOpts{
			Stdin:                    string(cmd.Stdin),
			SkipEntrypoint:           !thunk.UseEntrypoint,
			InsecureRootCapabilities: thunk.Insecure,
		})
	} else if forceExec {
		ctr = ctr.WithExec(append(thunk.Entrypoint, thunk.DefaultArgs...))
	}

	if len(thunk.Entrypoint) > 0 || thunk.ClearEntrypoint {
		ctr = ctr.WithEntrypoint(thunk.Entrypoint)
	}

	if len(thunk.DefaultArgs) > 0 || thunk.ClearDefaultArgs {
		ctr = ctr.WithDefaultArgs(thunk.DefaultArgs)
	}

	return ctr, nil
}

var epoch = time.Date(1985, 10, 26, 8, 15, 0, 0, time.UTC)

func (runtime *Dagger) mount(ctx context.Context, ctr *dagger.Container, target string, src bass.ThunkMountSource) (*dagger.Container, error) {
	switch {
	case src.ThunkPath != nil:
		srcCtr, err := runtime.Container(ctx, src.ThunkPath.Thunk, true)
		if err != nil {
			return nil, err
		}

		fsp := src.ThunkPath.Path.FilesystemPath()
		if fsp.IsDir() {
			return ctr.WithMountedDirectory(
				target,
				daggerGlob(srcCtr.Directory(fsp.Slash()), fsp).
					WithTimestamps(int(epoch.Unix())),
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
			dag.CacheVolume(src.Cache.ID),
			dagger.ContainerWithMountedCacheOpts{
				Sharing: mode,
			},
		), nil
	case src.FSPath != nil:
		dir := dag.Directory()

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
			return ctr.WithMountedDirectory(
				target,
				daggerGlob(dir.Directory(fsp.Slash()), fsp),
			), nil
		} else {
			return ctr.WithMountedFile(target, dir.File(fsp.Slash())), nil
		}
	case src.HostPath != nil:
		dir := dag.Host().Directory(src.HostPath.ContextDir, dagger.HostDirectoryOpts{
			Include: src.HostPath.Includes(),
			Exclude: src.HostPath.Excludes(),
		})
		fsp := src.HostPath.Path.FilesystemPath()

		if fsp.IsDir() {
			return ctr.WithMountedDirectory(target, dir.Directory(fsp.FromSlash())), nil
		} else {
			return ctr.WithMountedFile(target, dir.File(fsp.FromSlash())), nil
		}
	case src.Secret != nil:
		secret := dag.SetSecret(src.Secret.Name, string(src.Secret.Reveal()))
		return ctr.WithMountedSecret(target, secret), nil
	default:
		return nil, fmt.Errorf("mounting %T not implemented yet", src.ToValue())
	}
}

var daggerOCICache = newProtoCache[*dagger.Container]()

func basics(ctr *dagger.Container) *dagger.Container {
	return ctr.
		WithMountedTemp("/tmp").
		WithMountedTemp("/dev/shm").
		WithWorkdir(workDir)
}

func (runtime *Dagger) image(ctx context.Context, image *bass.ThunkImage) (*dagger.Container, error) {
	switch {
	case image == nil:
		return dag.Container(), nil

	case image.Ref != nil:
		ref, err := image.Ref.Ref()
		if err != nil {
			return nil, err
		}

		return basics(dag.Container().From(ref)), nil

	case image.Thunk != nil:
		ctr, err := runtime.Container(ctx, *image.Thunk, false)
		if err != nil {
			return nil, fmt.Errorf("image thunk: %w", err)
		}

		return ctr, nil

	case image.Archive != nil:
		cachedCtr, found := daggerOCICache.Get(ctx, image.Archive)
		if found {
			return cachedCtr, nil
		}

		archive := image.Archive

		file, err := runtime.inputFile(ctx, archive.File)
		if err != nil {
			return nil, fmt.Errorf("image thunk: %w", err)
		}

		name := archive.File.ToValue().String()
		if archive.Tag != "" {
			name += ":" + archive.Tag
		}

		ctr := basics(dag.Container(dagger.ContainerOpts{
			Platform: dagger.Platform(archive.Platform.String()),
		}).
			Import(file))

		daggerOCICache.Put(ctx, image.Archive, ctr)

		return ctr, nil

	case image.DockerBuild != nil:
		build := image.DockerBuild

		context, err := runtime.inputDirectory(ctx, build.Context)
		if err != nil {
			return nil, fmt.Errorf("image build input context: %w", err)
		}

		opts := dagger.ContainerBuildOpts{
			Target: build.Target,
		}

		if build.Dockerfile != nil {
			opts.Dockerfile = build.Dockerfile.Slash()
		}

		if build.Args != nil {
			_ = build.Args.Each(func(k bass.Symbol, v bass.Value) error {
				var str string
				if err := v.Decode(&str); err != nil {
					str = v.String()
				}

				opts.BuildArgs = append(opts.BuildArgs, dagger.BuildArg{
					Name:  k.String(),
					Value: str,
				})

				return nil
			})
		}

		ctr := basics(dag.Container(dagger.ContainerOpts{
			Platform: dagger.Platform(build.Platform.String()),
		}).Build(context, opts))

		return ctr, nil

	default:
		return nil, fmt.Errorf("unsupported image type: %s", image.ToValue())
	}
}

func (runtime *Dagger) inputFile(ctx context.Context, input bass.ImageBuildInput) (*dagger.File, error) {
	root, fsp, err := runtime.inputRoot(ctx, input)
	if err != nil {
		return nil, err
	}
	return root.File(fsp.Slash()), nil
}

func (runtime *Dagger) inputDirectory(ctx context.Context, input bass.ImageBuildInput) (*dagger.Directory, error) {
	root, fsp, err := runtime.inputRoot(ctx, input)
	if err != nil {
		return nil, err
	}
	return daggerGlob(root.Directory(fsp.Slash()), fsp), nil
}

func (runtime *Dagger) inputRoot(ctx context.Context, input bass.ImageBuildInput) (interface {
	File(string) *dagger.File
	Directory(string) *dagger.Directory
}, bass.FilesystemPath, error) {
	switch {
	case input.Thunk != nil:
		srcCtr, err := runtime.Container(ctx, input.Thunk.Thunk, true)
		if err != nil {
			return nil, nil, fmt.Errorf("image thunk: %w", err)
		}

		return srcCtr, input.Thunk.Path.FilesystemPath(), nil
	case input.Host != nil:
		dir := dag.Host().Directory(input.Host.ContextDir)
		fsp := input.Host.Path.FilesystemPath()
		return dir, fsp, nil
	case input.FS != nil:
		dir := dag.Directory()

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
			return nil, nil, fmt.Errorf("walk %s: %w", root, err)
		}

		fsp := input.FS.Path.FilesystemPath()
		return dir, fsp, nil
	default:
		return nil, nil, fmt.Errorf("unknown input type: %T", input.ToValue())
	}
}

func daggerGlob(dir *dagger.Directory, fsp bass.FilesystemPath) *dagger.Directory {
	if glob, ok := fsp.(bass.Globbable); ok {
		includes := glob.Includes()
		excludes := glob.Excludes()
		if len(includes) > 0 || len(excludes) > 0 {
			dir = dag.Directory().WithDirectory(".", dir, dagger.DirectoryWithDirectoryOpts{
				Include: includes,
				Exclude: excludes,
			})
		}
	}
	return dir
}
