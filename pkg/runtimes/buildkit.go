package runtimes

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/adrg/xdg"
	"github.com/containerd/containerd/platforms"
	dockerconfig "github.com/docker/cli/cli/config"
	"github.com/docker/distribution/reference"
	"github.com/hashicorp/go-multierror"
	"github.com/moby/buildkit/client"
	kitdclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
	gwclient "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/morikuni/aec"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/tonistiigi/units"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstls"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/runtimes/util/buildkitd"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/progrock"
	"github.com/vito/progrock/graph"
	"go.uber.org/zap"

	_ "embed"
)

const buildkitProduct = "bass"

type BuildkitConfig struct {
	Addr         string `json:"addr,omitempty"`
	DisableCache bool   `json:"disable_cache,omitempty"`
	CertsDir     string `json:"certs_dir,omitempty"`
}

var _ bass.Runtime = &Buildkit{}

//go:embed bin/exe.*
var shims embed.FS

const BuildkitName = "buildkit"

const shimExePath = "/bass/shim"
const workDir = "/bass/work"
const ioDir = "/bass/io"
const inputFile = "/bass/io/in"
const outputFile = "/bass/io/out"
const caFile = "/bass/ca.crt"

const digestBucket = "_digests"
const configBucket = "_configs"

var allShims = map[string][]byte{}

func init() {
	RegisterRuntime(BuildkitName, NewBuildkit)

	files, err := shims.ReadDir("bin")
	if err == nil {
		for _, f := range files {
			content, err := shims.ReadFile(path.Join("bin", f.Name()))
			if err == nil {
				allShims[f.Name()] = content
			}
		}
	}
}

type Buildkit struct {
	Config   BuildkitConfig
	Client   *kitdclient.Client
	Platform ocispecs.Platform

	authp session.Attachable
}

func NewBuildkit(ctx context.Context, _ bass.RuntimePool, cfg *bass.Scope) (bass.Runtime, error) {
	var config BuildkitConfig
	if cfg != nil {
		if err := cfg.Decode(&config); err != nil {
			return nil, fmt.Errorf("docker runtime config: %w", err)
		}
	}

	if config.CertsDir == "" {
		config.CertsDir = basstls.DefaultDir
	}

	client, err := dialBuildkit(ctx, config.Addr)
	if err != nil {
		return nil, fmt.Errorf("dial buildkit: %w", err)
	}

	workers, err := client.ListWorkers(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("list buildkit workers: %w", err)
	}

	var platform ocispecs.Platform
	var checkSame platforms.Matcher
	for _, w := range workers {
		if checkSame != nil && !checkSame.Match(w.Platforms[0]) {
			return nil, fmt.Errorf("TODO: workers have different platforms: %s != %s", w.Platforms[0], platform)
		}

		platform = w.Platforms[0]
		checkSame = platforms.Only(platform)
	}

	return &Buildkit{
		Config:   config,
		Client:   client,
		Platform: platform,

		authp: authprovider.NewDockerAuthProvider(dockerconfig.LoadDefaultConfigFile(os.Stderr)),
	}, nil
}

func dialBuildkit(ctx context.Context, addr string) (*kitdclient.Client, error) {
	if addr == "" {
		addr = os.Getenv("BUILDKIT_HOST")
	}

	if addr == "" {
		sockPath, err := xdg.SearchConfigFile("bass/buildkitd.sock")
		if err == nil {
			// support respecting XDG_RUNTIME_DIR instead of assuming /run/
			addr = "unix://" + sockPath
		}

		sockPath, err = xdg.SearchRuntimeFile("buildkit/buildkitd.sock")
		if err == nil {
			// support respecting XDG_RUNTIME_DIR instead of assuming /run/
			addr = "unix://" + sockPath
		}
	}

	var errs error
	if addr == "" {
		var startErr error
		addr, startErr = buildkitd.Start(ctx)
		if startErr != nil {
			errs = multierror.Append(startErr)
		}
	}

	client, err := kitdclient.New(context.TODO(), addr)
	if err != nil {
		errs = multierror.Append(errs, err)
		return nil, errs
	}

	return client, nil
}

func (runtime *Buildkit) Resolve(ctx context.Context, imageRef bass.ImageRef) (bass.ImageRef, error) {
	// track dependent services
	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()

	ref, err := runtime.ref(ctx, imageRef)
	if err != nil {
		// TODO: it might make sense to resolve an OCI archive ref to a digest too
		return bass.ImageRef{}, fmt.Errorf("resolve ref %v: %w", imageRef, err)
	}

	// convert 'ubuntu' to 'docker.io/library/ubuntu:latest'
	normalized, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return bass.ImageRef{}, fmt.Errorf("normalize ref: %w", err)
	}

	statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
	defer statusProxy.Wait()

	doBuild := func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
		digest, _, err := gw.ResolveImageConfig(ctx, normalized.String(), llb.ResolveImageConfigOpt{
			Platform: &runtime.Platform,
		})
		if err != nil {
			return nil, err
		}

		imageRef.Digest = digest.String()

		return &gwclient.Result{}, nil
	}

	_, err = runtime.Client.Build(ctx, kitdclient.SolveOpt{
		Session: []session.Attachable{
			runtime.authp,
		},
	}, buildkitProduct, doBuild, statusProxy.Writer())
	if err != nil {
		return bass.ImageRef{}, statusProxy.NiceError("resolve failed", err)
	}

	return imageRef, nil
}

func (runtime *Buildkit) Run(ctx context.Context, thunk bass.Thunk) error {
	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()
	return runtime.build(
		ctx,
		thunk,
		func(st llb.ExecState, _ string) marshalable {
			return st.GetMount(ioDir)
		},
		nil, // exports
	)
}

func (runtime *Buildkit) Start(ctx context.Context, thunk bass.Thunk) (StartResult, error) {
	ctx, stop := context.WithCancel(ctx)

	host := thunk.Name()

	health := runtime.newHealth(host, thunk.Ports)

	runs := bass.RunsFromContext(ctx)

	checked := make(chan error, 1)
	runs.Go(stop, func() error {
		checked <- health.Check(ctx)
		return nil
	})

	exited := make(chan error, 1)
	runs.Go(stop, func() error {
		exited <- runtime.build(
			ctx,
			thunk,
			func(st llb.ExecState, _ string) marshalable {
				return st.GetMount(ioDir)
			},
			nil,             // exports
			llb.IgnoreCache, // never cache services
		)

		return nil
	})

	select {
	case <-checked:
		result := StartResult{
			Ports: PortInfos{},
		}

		for _, port := range thunk.Ports {
			result.Ports[port.Name] = bass.Bindings{
				"host": bass.String(host),
				"port": bass.Int(port.Port),
			}.Scope()
		}

		return result, nil
	case err := <-exited:
		stop() // interrupt healthcheck

		if err != nil {
			return StartResult{}, err
		}

		return StartResult{}, fmt.Errorf("service exited before healthcheck")
	}
}

func (runtime *Buildkit) Read(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()

	hash, err := thunk.Hash()
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "thunk-"+hash)
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmp)

	err = runtime.build(
		ctx,
		thunk,
		func(st llb.ExecState, _ string) marshalable {
			return st.GetMount(ioDir)
		},
		[]kitdclient.ExportEntry{
			{
				Type:      kitdclient.ExporterLocal,
				OutputDir: tmp,
			},
		},
		llb.AddEnv("_BASS_OUTPUT", outputFile),
	)
	if err != nil {
		return err
	}

	response, err := os.Open(filepath.Join(tmp, filepath.Base(outputFile)))
	if err == nil {
		defer response.Close()

		_, err = io.Copy(w, response)
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}
	}

	return nil
}

type marshalable interface {
	Marshal(ctx context.Context, co ...llb.ConstraintsOpt) (*llb.Definition, error)
}

func (runtime *Buildkit) Export(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()
	return runtime.build(
		ctx,
		thunk,
		func(st llb.ExecState, _ string) marshalable { return st },
		[]kitdclient.ExportEntry{
			{
				Type: kitdclient.ExporterOCI,
				Output: func(map[string]string) (io.WriteCloser, error) {
					return nopCloser{w}, nil
				},
			},
		},
	)
}

func (runtime *Buildkit) ExportPath(ctx context.Context, w io.Writer, tp bass.ThunkPath) error {
	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()

	thunk := tp.Thunk
	path := tp.Path

	return runtime.build(
		ctx,
		thunk,
		func(st llb.ExecState, sp string) marshalable {
			copyOpt := &llb.CopyInfo{}
			if path.FilesystemPath().IsDir() {
				copyOpt.CopyDirContentsOnly = true
			}

			return llb.Scratch().File(
				llb.Copy(st.GetMount(workDir), filepath.Join(sp, path.FilesystemPath().FromSlash()), ".", copyOpt),
				llb.WithCustomNamef("[hide] copy %s", path.Slash()),
			)
		},
		[]kitdclient.ExportEntry{
			{
				Type: kitdclient.ExporterTar,
				Output: func(map[string]string) (io.WriteCloser, error) {
					return nopCloser{w}, nil
				},
			},
		},
	)
}

func (runtime *Buildkit) Prune(ctx context.Context, opts bass.PruneOpts) error {
	stderr := ioctx.StderrFromContext(ctx)
	tw := tabwriter.NewWriter(stderr, 2, 8, 2, ' ', 0)

	ch := make(chan kitdclient.UsageInfo)
	printed := make(chan struct{})

	total := int64(0)

	go func() {
		defer close(printed)
		for du := range ch {
			line := fmt.Sprintf("pruned %s", du.ID)
			if du.LastUsedAt != nil {
				line += fmt.Sprintf("\tuses: %d\tlast used: %s ago", du.UsageCount, time.Since(*du.LastUsedAt).Truncate(time.Second))
			}

			line += fmt.Sprintf("\tsize: %.2f", units.Bytes(du.Size))

			line += fmt.Sprintf("\t%s", aec.LightBlackF.Apply(du.Description))

			fmt.Fprintln(tw, line)

			total += du.Size
		}
	}()

	kitdOpts := []kitdclient.PruneOption{
		client.WithKeepOpt(opts.KeepDuration, opts.KeepBytes),
	}

	if opts.All {
		kitdOpts = append(kitdOpts, client.PruneAll)
	}

	err := runtime.Client.Prune(ctx, ch, kitdOpts...)
	close(ch)
	<-printed
	if err != nil {
		return err
	}

	fmt.Fprintf(tw, "total: %.2f\n", units.Bytes(total))

	return tw.Flush()
}

func (runtime *Buildkit) Close() error {
	return runtime.Client.Close()
}

func (runtime *Buildkit) build(
	ctx context.Context,
	thunk bass.Thunk,
	transform func(llb.ExecState, string) marshalable,
	exports []kitdclient.ExportEntry,
	runOpts ...llb.RunOption,
) error {
	var def *llb.Definition
	var secrets map[string][]byte
	var localDirs map[string]string
	var allowed []entitlements.Entitlement

	statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
	defer statusProxy.Wait()

	// build llb definition using the remote gateway for image resolution
	_, err := runtime.Client.Build(ctx, kitdclient.SolveOpt{
		Session: []session.Attachable{runtime.authp},
	}, buildkitProduct, func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
		b := runtime.newBuilder(ctx, gw)

		st, sp, needsInsecure, err := b.llb(ctx, thunk, runOpts...)
		if err != nil {
			return nil, err
		}

		if needsInsecure {
			allowed = append(allowed, entitlements.EntitlementSecurityInsecure)
		}

		localDirs = b.localDirs
		secrets = b.secrets

		def, err = transform(st, sp).Marshal(ctx)
		if err != nil {
			return nil, err
		}

		return &gwclient.Result{}, nil
	}, statusProxy.Writer())
	if err != nil {
		return statusProxy.NiceError("llb build failed", err)
	}

	_, err = runtime.Client.Solve(ctx, def, kitdclient.SolveOpt{
		LocalDirs:           localDirs,
		AllowedEntitlements: allowed,
		Session: []session.Attachable{
			runtime.authp,
			secretsprovider.FromMap(secrets),
		},
		Exports: exports,
	}, statusProxy.Writer())
	if err != nil {
		return statusProxy.NiceError("build failed", err)
	}

	return nil
}

func result(ctx context.Context, gw gwclient.Client, st marshalable) (*gwclient.Result, error) {
	def, err := st.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	return gw.Solve(ctx, gwclient.SolveRequest{
		Definition: def.ToPB(),
	})
}

type portHealthChecker struct {
	runtime *Buildkit

	host  string
	ports []bass.ThunkPort
}

func (runtime *Buildkit) newHealth(host string, ports []bass.ThunkPort) *portHealthChecker {
	return &portHealthChecker{
		runtime: runtime,

		host:  host,
		ports: ports,
	}
}

func (d *portHealthChecker) Check(ctx context.Context) error {
	_, err := d.runtime.Client.Build(ctx, kitdclient.SolveOpt{
		Session: []session.Attachable{
			d.runtime.authp,
		},
	}, buildkitProduct, d.doBuild, nil)
	return err
}

func (d *portHealthChecker) doBuild(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
	shimExe, err := d.runtime.shim()
	if err != nil {
		return nil, err
	}

	shimRes, err := result(ctx, gw, shimExe)
	if err != nil {
		return nil, err
	}

	scratchRes, err := result(ctx, gw, llb.Scratch())
	if err != nil {
		return nil, err
	}

	container, err := gw.NewContainer(ctx, gwclient.NewContainerRequest{
		Mounts: []gwclient.Mount{
			{
				Dest:      "/",
				MountType: pb.MountType_BIND,
				Ref:       scratchRes.Ref,
			},
			{
				Dest:      shimExePath,
				MountType: pb.MountType_BIND,
				Ref:       shimRes.Ref,
				Selector:  "run",
			},
		},
	})
	if err != nil {
		return nil, err
	}

	defer container.Release(ctx)

	args := []string{shimExePath, "check", d.host}
	for _, port := range d.ports {
		args = append(args, fmt.Sprintf("%s:%d", port.Name, port.Port))
	}

	proc, err := container.Start(ctx, gwclient.StartRequest{
		Args:   args,
		Stderr: nopCloser{ioctx.StderrFromContext(ctx)},
	})
	if err != nil {
		return nil, err
	}

	err = proc.Wait()
	if err != nil {
		return nil, err
	}

	return &gwclient.Result{}, nil
}

type builder struct {
	runtime  *Buildkit
	resolver llb.ImageMetaResolver

	secrets   map[string][]byte
	localDirs map[string]string
}

func (runtime *Buildkit) newBuilder(ctx context.Context, resolver llb.ImageMetaResolver) *builder {
	return &builder{
		runtime:  runtime,
		resolver: resolver,

		secrets:   map[string][]byte{},
		localDirs: map[string]string{},
	}
}

func (b *builder) llb(ctx context.Context, thunk bass.Thunk, extraOpts ...llb.RunOption) (llb.ExecState, string, bool, error) {
	cmd, err := NewCommand(ctx, b.runtime, thunk)
	if err != nil {
		return llb.ExecState{}, "", false, err
	}

	imageRef, runState, sourcePath, needsInsecure, err := b.image(ctx, thunk.Image)
	if err != nil {
		return llb.ExecState{}, "", false, err
	}

	id, err := thunk.Hash()
	if err != nil {
		return llb.ExecState{}, "", false, err
	}

	cmdPayload, err := bass.MarshalJSON(cmd)
	if err != nil {
		return llb.ExecState{}, "", false, err
	}

	shimExe, err := b.runtime.shim()
	if err != nil {
		return llb.ExecState{}, "", false, err
	}

	rootCA, err := os.ReadFile(basstls.CACert(b.runtime.Config.CertsDir))
	if err != nil {
		return llb.ExecState{}, "", false, err
	}

	runOpt := []llb.RunOption{
		llb.WithCustomName(thunk.Cmdline()),
		// NB: this is load-bearing; it's what busts the cache with different labels
		llb.Hostname(id),
		llb.AddMount("/tmp", llb.Scratch(), llb.Tmpfs()),
		llb.AddMount("/dev/shm", llb.Scratch(), llb.Tmpfs()),
		llb.AddMount(ioDir, llb.Scratch().File(
			llb.Mkfile("in", 0600, cmdPayload),
			llb.WithCustomName("[hide] mount command json"),
		)),
		llb.AddMount(shimExePath, shimExe, llb.SourcePath("run")),
		llb.AddMount(caFile, llb.Scratch().File(
			llb.Mkfile("ca.crt", 0600, rootCA),
			llb.WithCustomName("[hide] mount bass ca"),
		), llb.SourcePath("ca.crt")),
		llb.With(llb.Dir(workDir)),
		llb.Args([]string{shimExePath, "run", inputFile}),
	}

	if thunk.TLS != nil {
		crt, key, err := basstls.Generate(b.runtime.Config.CertsDir, id)
		if err != nil {
			return llb.ExecState{}, "", false, fmt.Errorf("tls: generate: %w", err)
		}

		crtContent, err := crt.Export()
		if err != nil {
			return llb.ExecState{}, "", false, fmt.Errorf("export crt: %w", err)
		}

		keyContent, err := key.ExportPrivate()
		if err != nil {
			return llb.ExecState{}, "", false, fmt.Errorf("export key: %w", err)
		}

		runOpt = append(runOpt,
			llb.AddMount(
				thunk.TLS.Cert.FromSlash(),
				llb.Scratch().File(
					llb.Mkfile(thunk.TLS.Cert.Name(), 0600, crtContent),
					llb.WithCustomName("[hide] mount thunk tls cert"),
				),
				llb.SourcePath(thunk.TLS.Cert.Name()),
			),
			llb.AddMount(
				thunk.TLS.Key.FromSlash(),
				llb.Scratch().File(
					llb.Mkfile(thunk.TLS.Key.Name(), 0600, keyContent),
					llb.WithCustomName("[hide] mount thunk tls key"),
				),
				llb.SourcePath(thunk.TLS.Key.Name()),
			),
		)
	}

	if thunk.Insecure {
		needsInsecure = true

		runOpt = append(runOpt,
			llb.WithCgroupParent(id),
			llb.Security(llb.SecurityModeInsecure))
	}

	var remountedWorkdir bool
	for _, mount := range cmd.Mounts {
		var targetPath string
		if filepath.IsAbs(mount.Target) {
			targetPath = mount.Target
		} else {
			targetPath = filepath.Join(workDir, mount.Target)
		}

		mountOpt, sp, ni, err := b.initializeMount(ctx, mount.Source, targetPath)
		if err != nil {
			return llb.ExecState{}, "", false, err
		}

		if targetPath == workDir {
			remountedWorkdir = true
			sourcePath = sp
		}

		if ni {
			needsInsecure = true
		}

		runOpt = append(runOpt, mountOpt)
	}

	if !remountedWorkdir {
		if sourcePath != "" {
			// NB: could just call SourcePath with "", but this is to ensure there's
			// code coverage
			runOpt = append(runOpt, llb.AddMount(workDir, runState, llb.SourcePath(sourcePath)))
		} else {
			runOpt = append(runOpt, llb.AddMount(workDir, runState))
		}
	}

	if len(thunk.Ports) > 0 || b.runtime.Config.DisableCache {
		runOpt = append(runOpt, llb.IgnoreCache)
	}

	runOpt = append(runOpt, extraOpts...)

	return imageRef.Run(runOpt...), sourcePath, needsInsecure, nil
}

func (runtime *Buildkit) shim() (llb.State, error) {
	shimExe, found := allShims["exe."+runtime.Platform.Architecture]
	if !found {
		return llb.State{}, fmt.Errorf("no shim found for %s", runtime.Platform.Architecture)
	}

	return llb.Scratch().File(
		llb.Mkfile("/run", 0755, shimExe),
		llb.WithCustomName("[hide] load bass shim"),
	), nil
}

func (r *Buildkit) ref(ctx context.Context, imageRef bass.ImageRef) (string, error) {
	if imageRef.Repository.Addr != nil {
		addr := imageRef.Repository.Addr

		result, err := r.Start(ctx, addr.Thunk)
		if err != nil {
			return "", err
		}

		info, found := result.Ports[addr.Port]
		if !found {
			zapctx.FromContext(ctx).Error("unknown port",
				zap.Any("thunk", addr.Thunk),
				zap.Any("ports", result.Ports))
			return "", fmt.Errorf("unknown port: %s", addr.Port)
		}

		repo, err := addr.Render(info)
		if err != nil {
			return "", err
		}

		imageRef.Repository.Static = repo
	}

	return imageRef.Ref()
}

func (b *builder) image(ctx context.Context, image *bass.ThunkImage) (llb.State, llb.State, string, bool, error) {
	if image == nil {
		// TODO: test
		return llb.Scratch(), llb.Scratch(), "", false, nil
	}

	if image.Ref != nil {
		ref, err := b.runtime.ref(ctx, *image.Ref)
		if err != nil {
			return llb.State{}, llb.State{}, "", false, err
		}

		return llb.Image(
			ref,
			llb.WithMetaResolver(b.resolver),
			llb.Platform(b.runtime.Platform),
		), llb.Scratch(), "", false, nil
	}

	if image.Thunk != nil {
		execState, sourcePath, needsInsecure, err := b.llb(ctx, *image.Thunk)
		if err != nil {
			return llb.State{}, llb.State{}, "", false, fmt.Errorf("image thunk llb: %w", err)
		}

		return execState.State, execState.GetMount(workDir), sourcePath, needsInsecure, nil
	}

	if image.Archive != nil {
		return b.unpackImageArchive(ctx, image.Archive.File, image.Archive.Tag)
	}

	return llb.State{}, llb.State{}, "", false, fmt.Errorf("unsupported image type: %+v", image)
}

func (b *builder) unpackImageArchive(ctx context.Context, thunkPath bass.ThunkPath, tag string) (llb.State, llb.State, string, bool, error) {
	shimExe, err := b.runtime.shim()
	if err != nil {
		return llb.State{}, llb.State{}, "", false, err
	}

	thunkSt, baseSourcePath, needsInsecure, err := b.llb(ctx, thunkPath.Thunk)
	if err != nil {
		return llb.State{}, llb.State{}, "", false, fmt.Errorf("thunk llb: %w", err)
	}

	sourcePath := filepath.Join(baseSourcePath, thunkPath.Path.FilesystemPath().FromSlash())

	configSt := llb.Scratch().Run(
		llb.AddMount("/shim", shimExe, llb.SourcePath("run")),
		llb.AddMount(
			"/image.tar",
			thunkSt.GetMount(workDir),
			llb.SourcePath(sourcePath),
		),
		llb.AddMount("/config", llb.Scratch()),
		llb.Args([]string{"/shim", "get-config", "/image.tar", tag, "/config"}),
	)

	unpackSt := llb.Scratch().Run(
		llb.AddMount("/shim", shimExe, llb.SourcePath("run")),
		llb.AddMount(
			"/image.tar",
			thunkSt.GetMount(workDir),
			llb.SourcePath(sourcePath),
		),
		llb.AddMount("/rootfs", llb.Scratch()),
		llb.Args([]string{"/shim", "unpack", "/image.tar", tag, "/rootfs"}),
	)

	image := unpackSt.GetMount("/rootfs")

	var allowed []entitlements.Entitlement
	if needsInsecure {
		allowed = append(allowed, entitlements.EntitlementSecurityInsecure)
	}

	statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
	defer statusProxy.Wait()

	_, err = b.runtime.Client.Build(ctx, kitdclient.SolveOpt{
		LocalDirs:           b.localDirs,
		AllowedEntitlements: allowed,
		Session: []session.Attachable{
			b.runtime.authp,
			secretsprovider.FromMap(b.secrets),
		},
	}, buildkitProduct, func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
		def, err := configSt.GetMount("/config").Marshal(ctx, llb.WithCaps(gw.BuildOpts().LLBCaps))
		if err != nil {
			return nil, err
		}

		res, err := gw.Solve(ctx, gwclient.SolveRequest{
			Definition: def.ToPB(),
		})
		if err != nil {
			return nil, err
		}

		singleRef, err := res.SingleRef()
		if err != nil {
			return nil, fmt.Errorf("get single ref: %w", err)
		}

		cfg, err := singleRef.ReadFile(ctx, gwclient.ReadRequest{Filename: "/config.json"})
		if err != nil {
			return nil, fmt.Errorf("read config.json: %w", err)
		}

		var iconf ocispecs.ImageConfig
		err = json.Unmarshal(cfg, &iconf)
		if err != nil {
			return nil, fmt.Errorf("unmarshal runtime config: %w", err)
		}

		for _, env := range iconf.Env {
			parts := strings.SplitN(env, "=", 2)
			if len(parts[0]) > 0 {
				var v string
				if len(parts) > 1 {
					v = parts[1]
				}
				image = image.AddEnv(parts[0], v)
			}
		}

		return &gwclient.Result{}, nil
	}, statusProxy.Writer())
	if err != nil {
		return llb.State{}, llb.State{}, "", false, statusProxy.NiceError("oci unpack failed", err)
	}

	return image, llb.Scratch(), "", needsInsecure, nil
}

func (b *builder) initializeMount(ctx context.Context, source bass.ThunkMountSource, targetPath string) (llb.RunOption, string, bool, error) {
	if source.ThunkPath != nil {
		thunkSt, baseSourcePath, needsInsecure, err := b.llb(ctx, source.ThunkPath.Thunk)
		if err != nil {
			return nil, "", false, fmt.Errorf("thunk llb: %w", err)
		}

		sourcePath := filepath.Join(baseSourcePath, source.ThunkPath.Path.FilesystemPath().FromSlash())

		return llb.AddMount(
			targetPath,
			thunkSt.GetMount(workDir),
			llb.SourcePath(sourcePath),
		), sourcePath, needsInsecure, nil
	}

	if source.HostPath != nil {
		contextDir := source.HostPath.ContextDir
		b.localDirs[contextDir] = source.HostPath.ContextDir

		var excludes []string
		ignorePath := filepath.Join(contextDir, ".bassignore")
		ignore, err := os.Open(ignorePath)
		if err == nil {
			excludes, err = dockerignore.ReadAll(ignore)
			if err != nil {
				return nil, "", false, fmt.Errorf("parse %s: %w", ignorePath, err)
			}
		}

		sourcePath := source.HostPath.Path.FilesystemPath().FromSlash()

		return llb.AddMount(
			targetPath,
			llb.Scratch().File(llb.Copy(
				llb.Local(
					contextDir,
					llb.ExcludePatterns(excludes),
					llb.Differ(llb.DiffMetadata, false),
				),
				sourcePath, // allow fine-grained caching control
				sourcePath,
				&llb.CopyInfo{
					CopyDirContentsOnly: true,
					CreateDestPath:      true,
				},
			)),
			llb.SourcePath(sourcePath),
		), sourcePath, false, nil
	}

	if source.FSPath != nil {
		fsp := source.FSPath
		sourcePath := fsp.Path.FilesystemPath().FromSlash()

		if fsp.Path.File != nil {
			content, err := fs.ReadFile(fsp.FS, path.Clean(fsp.Path.Slash()))
			if err != nil {
				return nil, "", false, err
			}

			tree := llb.Scratch()

			filePath := path.Clean(fsp.Path.Slash())
			if strings.Contains(filePath, "/") {
				tree = tree.File(llb.Mkdir(path.Dir(filePath), 0755, llb.WithParents(true)))
			}

			return llb.AddMount(
				targetPath,
				tree.File(llb.Mkfile(filePath, 0644, content)),
				llb.SourcePath(sourcePath),
			), sourcePath, false, nil
		} else {
			tree := llb.Scratch()

			err := fs.WalkDir(fsp.FS, path.Clean(fsp.Path.Slash()), func(walkPath string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				info, err := d.Info()
				if err != nil {
					return err
				}

				if d.IsDir() {
					tree = tree.File(llb.Mkdir(walkPath, info.Mode(), llb.WithParents(true)))
				} else {
					content, err := fs.ReadFile(fsp.FS, walkPath)
					if err != nil {
						return fmt.Errorf("read %s: %w", walkPath, err)
					}

					if strings.Contains(walkPath, "/") {
						tree = tree.File(
							llb.Mkdir(path.Dir(walkPath), 0755, llb.WithParents(true)),
						)
					}

					tree = tree.File(llb.Mkfile(walkPath, info.Mode(), content))
				}

				return nil
			})
			if err != nil {
				return nil, "", false, fmt.Errorf("walk %s: %w", fsp, err)
			}

			return llb.AddMount(
				targetPath,
				tree,
				llb.SourcePath(sourcePath),
			), sourcePath, false, nil
		}
	}

	if source.Cache != nil {
		return llb.AddMount(
			targetPath,
			llb.Scratch(),
			llb.AsPersistentCacheDir(source.Cache.ID, llb.CacheMountLocked),
			llb.SourcePath(source.Cache.Path.FilesystemPath().FromSlash()),
		), "", false, nil
	}

	if source.Secret != nil {
		id := source.Secret.Name
		b.secrets[id] = source.Secret.Reveal()
		return llb.AddSecret(targetPath, llb.SecretID(id)), "", false, nil
	}

	return nil, "", false, fmt.Errorf("unrecognized mount source: %s", source.ToValue())
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

func forwardStatus(rec *progrock.Recorder) *statusProxy {
	return &statusProxy{
		rec:  rec,
		wg:   new(sync.WaitGroup),
		prog: cli.NewProgress(),
	}
}

// a bit of a cludge; this translates buildkit progress messages to progrock
// status messages, and also records the progress so that we can emit it in a
// friendlier error message
type statusProxy struct {
	rec  *progrock.Recorder
	wg   *sync.WaitGroup
	prog *cli.Progress
}

func (proxy *statusProxy) proxy(rec *progrock.Recorder, statuses chan *kitdclient.SolveStatus) {
	for {
		s, ok := <-statuses
		if !ok {
			break
		}

		vs := make([]*graph.Vertex, len(s.Vertexes))
		for i, v := range s.Vertexes {
			// TODO: we have strayed from upstream Buildkit, and it's tricky to
			// un-stray because now there are fields coupled to Buildkit types.
			vs[i] = &graph.Vertex{
				Digest:    v.Digest,
				Inputs:    v.Inputs,
				Name:      v.Name,
				Started:   v.Started,
				Completed: v.Completed,
				Cached:    v.Cached,
				Error:     v.Error,
			}
		}

		ss := make([]*graph.VertexStatus, len(s.Statuses))
		for i, s := range s.Statuses {
			ss[i] = (*graph.VertexStatus)(s)
		}

		ls := make([]*graph.VertexLog, len(s.Logs))
		for i, l := range s.Logs {
			ls[i] = (*graph.VertexLog)(l)
		}

		gstatus := &graph.SolveStatus{
			Vertexes: vs,
			Statuses: ss,
			Logs:     ls,
		}

		proxy.prog.WriteStatus(gstatus)
		rec.Record(gstatus)
	}
}

func (proxy *statusProxy) Writer() chan *kitdclient.SolveStatus {
	statuses := make(chan *kitdclient.SolveStatus)

	proxy.wg.Add(1)
	go func() {
		defer proxy.wg.Done()
		proxy.proxy(proxy.rec, statuses)
	}()

	return statuses
}

func (proxy *statusProxy) Wait() {
	proxy.wg.Wait()
}

func (proxy *statusProxy) NiceError(msg string, err error) bass.NiceError {
	return proxy.prog.WrapError(msg, err)
}
