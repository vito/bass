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
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/adrg/xdg"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/pkg/transfer/archive"
	"github.com/containerd/containerd/platforms"
	dockerconfig "github.com/docker/cli/cli/config"
	"github.com/docker/distribution/reference"
	"github.com/hashicorp/go-multierror"
	kitdclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
	"github.com/moby/buildkit/frontend/dockerui"
	gwclient "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/morikuni/aec"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/tonistiigi/units"
	"github.com/vito/progrock"
	"github.com/vito/progrock/graph"
	"go.uber.org/zap"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstls"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/runtimes/util/buildkitd"
	"github.com/vito/bass/pkg/zapctx"
)

const buildkitProduct = "bass"

const ociStoreName = "bass"

// OCI manifest annotation that specifies an image's tag
const ociTagAnnotation = "org.opencontainers.image.ref.name"

type BuildkitConfig struct {
	Debug        bool   `json:"debug,omitempty"`
	Addr         string `json:"addr,omitempty"`
	Installation string `json:"installation,omitempty"`
	DisableCache bool   `json:"disable_cache,omitempty"`
	CertsDir     string `json:"certs_dir,omitempty"`
	OCIStoreDir  string `json:"oci_store_dir,omitempty"`
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

	ociStore content.Store
}

const DefaultBuildkitInstallation = "bass-buildkitd"

func NewBuildkit(ctx context.Context, _ bass.RuntimePool, cfg *bass.Scope) (bass.Runtime, error) {
	var config BuildkitConfig
	if cfg != nil {
		if err := cfg.Decode(&config); err != nil {
			return nil, fmt.Errorf("buildkit runtime config: %w", err)
		}
	}

	if config.CertsDir == "" {
		config.CertsDir = basstls.DefaultDir
	}

	if config.Installation == "" {
		config.Installation = DefaultBuildkitInstallation
	}

	err := basstls.Init(config.CertsDir)
	if err != nil {
		return nil, fmt.Errorf("init tls depot: %w", err)
	}

	if config.OCIStoreDir == "" {
		config.OCIStoreDir = filepath.Join(xdg.DataHome, "bass", "oci")
	}

	client, err := dialBuildkit(ctx, config.Addr, config.Installation, config.CertsDir)
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

	store, err := local.NewStore(config.OCIStoreDir)
	if err != nil {
		return nil, fmt.Errorf("create oci store: %w", err)
	}

	return &Buildkit{
		Config:   config,
		Client:   client,
		Platform: platform,

		authp: authprovider.NewDockerAuthProvider(dockerconfig.LoadDefaultConfigFile(os.Stderr)),

		ociStore: store,
	}, nil
}

func dialBuildkit(ctx context.Context, addr string, installation string, certsDir string) (*kitdclient.Client, error) {
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
		addr, startErr = buildkitd.Start(ctx, installation, certsDir)
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
	_, err := runtime.build(
		ctx,
		thunk,
		func(st llb.ExecState, _ string) llb.State {
			return st.GetMount(ioDir)
		},
		nil, // exports
	)
	return err
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
		_, err := runtime.build(
			ctx,
			thunk,
			func(st llb.ExecState, _ string) llb.State {
				return st.GetMount(ioDir)
			},
			nil, // exports
		)

		exited <- err

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

	_, err = runtime.build(
		ctx,
		thunk,
		func(st llb.ExecState, _ string) llb.State {
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
	_, err := runtime.build(
		ctx,
		thunk,
		func(st llb.ExecState, _ string) llb.State {
			return st.Root()
		},
		[]kitdclient.ExportEntry{
			{
				Type: kitdclient.ExporterOCI,
				Output: func(map[string]string) (io.WriteCloser, error) {
					return nopCloser{w}, nil
				},
			},
		},
	)
	return err
}

func (runtime *Buildkit) Publish(ctx context.Context, ref bass.ImageRef, thunk bass.Thunk) (bass.ImageRef, error) {
	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()

	addr, err := ref.Ref()
	if err != nil {
		return ref, err
	}

	res, err := runtime.build(
		ctx,
		thunk,
		func(st llb.ExecState, _ string) llb.State {
			return st.Root()
		},
		[]kitdclient.ExportEntry{
			{
				Type: kitdclient.ExporterImage,
				Attrs: map[string]string{
					"name": addr,
					"push": "true",
				},
			},
		},
	)
	if err != nil {
		return ref, err
	}

	imageDigest, found := res.ExporterResponse[exptypes.ExporterImageDigestKey]
	if found {
		ref.Digest = imageDigest
	}

	return ref, nil
}

func (runtime *Buildkit) ExportPath(ctx context.Context, w io.Writer, tp bass.ThunkPath) error {
	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()

	thunk := tp.Thunk
	path := tp.Path

	_, err := runtime.build(
		ctx,
		thunk,
		func(st llb.ExecState, sp string) llb.State {
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
	return err
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
		kitdclient.WithKeepOpt(opts.KeepDuration, opts.KeepBytes),
	}

	if opts.All {
		kitdOpts = append(kitdOpts, kitdclient.PruneAll)
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
	transform func(llb.ExecState, string) llb.State,
	exports []kitdclient.ExportEntry,
	runOpts ...llb.RunOption,
) (*kitdclient.SolveResponse, error) {
	var def *llb.Definition
	var secrets map[string][]byte
	var localDirs map[string]string
	var imageConfig ocispecs.ImageConfig
	var allowed []entitlements.Entitlement

	statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
	defer statusProxy.Wait()

	// build llb definition using the remote gateway for image resolution
	_, err := runtime.Client.Build(ctx, kitdclient.SolveOpt{
		Session: []session.Attachable{runtime.authp},
	}, buildkitProduct, func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
		b := runtime.newBuilder(ctx, gw)

		ib, err := b.llb(ctx, thunk, transform, runOpts...)
		if err != nil {
			return nil, err
		}

		if ib.needsInsecure {
			allowed = append(allowed, entitlements.EntitlementSecurityInsecure)
		}

		localDirs = b.localDirs
		secrets = b.secrets
		imageConfig = ib.config

		def, err = ib.output.Marshal(ctx)
		if err != nil {
			return nil, err
		}

		return &gwclient.Result{}, nil
	}, statusProxy.Writer())
	if err != nil {
		return nil, statusProxy.NiceError("llb build failed", err)
	}

	res, err := runtime.Client.Build(ctx, kitdclient.SolveOpt{
		LocalDirs:           localDirs,
		AllowedEntitlements: allowed,
		Session: []session.Attachable{
			runtime.authp,
			secretsprovider.FromMap(secrets),
		},
		Exports: exports,
		OCIStores: map[string]content.Store{
			ociStoreName: runtime.ociStore,
		},
	}, buildkitProduct, func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
		res, err := gw.Solve(ctx, gwclient.SolveRequest{
			Evaluate:   true,
			Definition: def.ToPB(),
		})
		if err != nil {
			return nil, err
		}

		cfgBytes, err := json.Marshal(ocispecs.Image{
			Architecture: runtime.Platform.Architecture,
			OS:           runtime.Platform.OS,
			OSVersion:    runtime.Platform.OSVersion,
			OSFeatures:   runtime.Platform.OSFeatures,
			Config:       imageConfig,
		})
		if err != nil {
			return nil, err
		}
		res.AddMeta(exptypes.ExporterImageConfigKey, cfgBytes)

		return res, nil
	}, statusProxy.Writer())
	if err != nil {
		return nil, statusProxy.NiceError("build failed", err)
	}

	return res, nil
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

	// NB: use a different ctx than the one that'll be interrupted for anything
	// that needs to run as part of post-interruption cleanup
	cleanupCtx := context.Background()

	defer container.Release(cleanupCtx)

	args := []string{shimExePath, "check", d.host}
	for _, port := range d.ports {
		args = append(args, fmt.Sprintf("%s:%d", port.Name, port.Port))
	}

	proc, err := container.Start(cleanupCtx, gwclient.StartRequest{
		Args:   args,
		Stdout: nopCloser{ioctx.StderrFromContext(ctx)},
		Stderr: nopCloser{ioctx.StderrFromContext(ctx)},
	})
	if err != nil {
		return nil, err
	}

	exited := make(chan error, 1)
	go func() {
		exited <- proc.Wait()
	}()

	select {
	case err := <-exited:
		if err != nil {
			return nil, err
		}

		return &gwclient.Result{}, nil
	case <-ctx.Done():
		err := proc.Signal(cleanupCtx, syscall.SIGKILL)
		if err != nil {
			return nil, fmt.Errorf("interrupt check: %w", err)
		}

		<-exited

		return nil, ctx.Err()
	}
}

type buildkitBuilder struct {
	runtime  *Buildkit
	resolver llb.ImageMetaResolver

	secrets   map[string][]byte
	localDirs map[string]string
}

func (runtime *Buildkit) newBuilder(ctx context.Context, resolver llb.ImageMetaResolver) *buildkitBuilder {
	return &buildkitBuilder{
		runtime:  runtime,
		resolver: resolver,

		secrets:   map[string][]byte{},
		localDirs: map[string]string{},
	}
}

func (b *buildkitBuilder) llb(
	ctx context.Context,
	thunk bass.Thunk,
	transform func(llb.ExecState, string) llb.State,
	extraOpts ...llb.RunOption,
) (intermediateBuild, error) {
	ib, err := b.image(ctx, thunk.Image)
	if err != nil {
		return ib, err
	}

	thunkName, err := thunk.Hash()
	if err != nil {
		return ib, err
	}

	cmd, err := NewCommand(ctx, b.runtime, thunk)
	if err != nil {
		return ib, err
	}

	entrypoint := ib.config.Entrypoint

	// propagate thunk's entrypoint to the child
	if thunk.Entrypoint != nil { // note: nil vs. [] distinction
		ib.config.Entrypoint = thunk.Entrypoint
	}

	// propagate thunk's default command
	if thunk.DefaultArgs != nil { // note: nil vs. [] distinction
		ib.config.Cmd = thunk.DefaultArgs
	}

	if cmd.Args == nil { // note: nil vs. [] distinction
		// no command; we're just overriding config
		return ib, nil
	}

	cmd.Args = append(entrypoint, cmd.Args...)

	cmdPayload, err := bass.MarshalJSON(cmd)
	if err != nil {
		return ib, err
	}

	shimExe, err := b.runtime.shim()
	if err != nil {
		return ib, err
	}

	rootCA, err := os.ReadFile(basstls.CACert(b.runtime.Config.CertsDir))
	if err != nil {
		return ib, err
	}

	runOpt := []llb.RunOption{
		llb.WithCustomName(thunk.Cmdline()),
		// NB: this is load-bearing; it's what busts the cache with different labels
		llb.Hostname(thunkName),
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
		crt, key, err := basstls.Generate(b.runtime.Config.CertsDir, thunkName)
		if err != nil {
			return ib, fmt.Errorf("tls: generate: %w", err)
		}

		crtContent, err := crt.Export()
		if err != nil {
			return ib, fmt.Errorf("export crt: %w", err)
		}

		keyContent, err := key.ExportPrivate()
		if err != nil {
			return ib, fmt.Errorf("export key: %w", err)
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

	if b.runtime.Config.Debug {
		runOpt = append(runOpt, llb.AddEnv("_BASS_DEBUG", "1"))
	}

	for _, env := range cmd.SecretEnv {
		id := env.Secret.Name
		b.secrets[id] = env.Secret.Reveal()
		runOpt = append(runOpt, llb.AddSecret(env.Name, llb.SecretID(id), llb.SecretAsEnv(true)))
	}

	if thunk.Insecure {
		ib.needsInsecure = true

		runOpt = append(runOpt,
			llb.WithCgroupParent(thunkName),
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
			return ib, err
		}

		if targetPath == workDir {
			remountedWorkdir = true
			ib.sourcePath = sp
		}

		if ni {
			ib.needsInsecure = true
		}

		runOpt = append(runOpt, mountOpt)
	}

	if !remountedWorkdir {
		if ib.sourcePath != "" {
			// NB: could just call SourcePath with "", but this is to ensure there's
			// code coverage
			runOpt = append(runOpt, llb.AddMount(workDir, ib.output, llb.SourcePath(ib.sourcePath)))
		} else {
			runOpt = append(runOpt, llb.AddMount(workDir, ib.output))
		}
	}

	if b.runtime.Config.DisableCache {
		runOpt = append(runOpt, llb.IgnoreCache)
	}

	runOpt = append(runOpt, extraOpts...)

	execSt := ib.fs.Run(runOpt...)
	ib.output = transform(execSt, ib.sourcePath)
	ib.fs = execSt.State

	return ib, nil
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

type intermediateBuild struct {
	fs llb.State

	output llb.State

	sourcePath    string
	needsInsecure bool

	config ocispecs.ImageConfig
}

func (b *buildkitBuilder) image(ctx context.Context, image *bass.ThunkImage) (ib intermediateBuild, err error) {
	switch {
	case image == nil:
		// TODO: test; how is this possible?
		ib.fs = llb.Scratch()
		ib.output = llb.Scratch()
		return ib, nil

	case image.Ref != nil:
		ref, err := b.runtime.ref(ctx, *image.Ref)
		if err != nil {
			return ib, err
		}

		r, err := reference.ParseNormalizedNamed(ref)
		if err == nil {
			r = reference.TagNameOnly(r)
			ref = r.String()
		}

		dgst, config, err := b.resolver.ResolveImageConfig(ctx, ref, llb.ResolveImageConfigOpt{
			ResolverType: llb.ResolverTypeRegistry,
			Platform:     &b.runtime.Platform,
			ResolveMode:  llb.ResolveModeDefault.String(),
		})
		if err != nil {
			return ib, err
		}

		if dgst != "" {
			r, err = reference.WithDigest(r, dgst)
			if err != nil {
				return ib, err
			}
			ref = r.String()
		}

		st := llb.Image(ref, llb.Platform(b.runtime.Platform))

		var img ocispecs.Image
		if err := json.Unmarshal(config, &img); err != nil {
			return ib, err
		}
		for _, env := range img.Config.Env {
			parts := strings.SplitN(env, "=", 2)
			if len(parts[0]) > 0 {
				var v string
				if len(parts) > 1 {
					v = parts[1]
				}
				st = st.AddEnv(parts[0], v)
			}
		}
		st = st.Dir(img.Config.WorkingDir)
		if img.Architecture != "" && img.OS != "" {
			st = st.Platform(ocispecs.Platform{
				OS:           img.OS,
				Architecture: img.Architecture,
				Variant:      img.Variant,
			})
		}

		ib.fs = st
		ib.output = llb.Scratch()
		ib.config = img.Config
		return ib, nil

	case image.Thunk != nil:
		return b.llb(ctx, *image.Thunk, getWorkdir)

	case image.Archive != nil:
		file, err := image.Archive.File.Open(ctx)
		if err != nil {
			return ib, fmt.Errorf("image archive file: %w", err)
		}

		defer file.Close()

		stream := archive.NewImageImportStream(file, "")

		desc, err := stream.Import(ctx, b.runtime.ociStore)
		if err != nil {
			return ib, fmt.Errorf("image archive import: %w", err)
		}

		// NB: the repository portion of this ref doesn't actually matter, but it's
		// pleasant to see something recognizable.
		dummyRepo := path.Join(image.Archive.File.Thunk.Name(), image.Archive.File.Name())

		indexBlob, err := content.ReadBlob(ctx, b.runtime.ociStore, desc)
		if err != nil {
			return ib, fmt.Errorf("image archive read blob: %w", err)
		}

		var idx ocispecs.Index
		err = json.Unmarshal(indexBlob, &idx)
		if err != nil {
			return ib, fmt.Errorf("image archive unmarshal index: %w", err)
		}

		platform := platforms.Only(b.runtime.Platform)
		tag := image.Archive.Tag

		for _, m := range idx.Manifests {
			if m.Platform != nil {
				if !platform.Match(*m.Platform) {
					// incompatible
					continue
				}
			}

			if tag != "" {
				if m.Annotations == nil {
					continue
				}

				manifestTag, found := m.Annotations[ociTagAnnotation]
				if !found || manifestTag != tag {
					continue
				}
			}

			st := llb.OCILayout(
				fmt.Sprintf("%s@%s", dummyRepo, m.Digest),
				llb.OCIStore("", ociStoreName),
				llb.Platform(b.runtime.Platform),
			)

			manifestBlob, err := content.ReadBlob(ctx, b.runtime.ociStore, m)
			if err != nil {
				return ib, fmt.Errorf("image archive read blob: %w", err)
			}

			var man ocispecs.Manifest
			err = json.Unmarshal(manifestBlob, &man)
			if err != nil {
				return ib, fmt.Errorf("image archive unmarshal manifest: %w", err)
			}

			configBlob, err := content.ReadBlob(ctx, b.runtime.ociStore, man.Config)
			if err != nil {
				return ib, fmt.Errorf("image archive read blob: %w", err)
			}

			st, err = st.WithImageConfig(configBlob)
			if err != nil {
				return ib, fmt.Errorf("image archive with image config: %w", err)
			}

			ib.fs = st
			ib.output = llb.Scratch()
			return ib, nil
		}

		return ib, fmt.Errorf("image archive had no matching manifest for platform %s and tag %s", platforms.Format(b.runtime.Platform), tag)
	}

	if image.DockerBuild != nil {
		platform := image.DockerBuild.Platform
		contextDir := image.DockerBuild.Context
		dockerfile := image.DockerBuild.Dockerfile
		target := image.DockerBuild.Target
		buildArgs := image.DockerBuild.Args

		ctxSt, sourcePath, needsInsecure, err := b.buildInput(ctx, contextDir)
		if err != nil {
			return ib, fmt.Errorf("image docker build input: %w", err)
		}

		opts := map[string]string{
			"platform":      platform.String(),
			"contextsubdir": sourcePath,
		}

		const defaultDockerfileName = "Dockerfile"

		if dockerfile.Path != "" {
			opts["filename"] = path.Join(sourcePath, dockerfile.Slash())
		} else {
			opts["filename"] = path.Join(sourcePath, defaultDockerfileName)
		}

		if target != "" {
			opts["target"] = target
		}

		if buildArgs != nil {
			err := buildArgs.Each(func(k bass.Symbol, v bass.Value) error {
				var val string
				if err := v.Decode(&val); err != nil {
					return err
				}

				opts["build-arg:"+k.String()] = val
				return nil
			})
			if err != nil {
				return ib, fmt.Errorf("docker build args: %w", err)
			}
		}

		ctxDef, err := ctxSt.Marshal(ctx) // TODO: platform?
		if err != nil {
			return ib, fmt.Errorf("docker build marshal: %w", err)
		}

		var allowed []entitlements.Entitlement
		if needsInsecure {
			allowed = append(allowed, entitlements.EntitlementSecurityInsecure)
		}

		inputs := map[string]*pb.Definition{
			dockerui.DefaultLocalNameContext:    ctxDef.ToPB(),
			dockerui.DefaultLocalNameDockerfile: ctxDef.ToPB(),
		}

		statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
		defer statusProxy.Wait()

		var st llb.State
		doBuild := func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
			res, err := gw.Solve(ctx, gwclient.SolveRequest{
				Frontend:       "dockerfile.v0",
				FrontendOpt:    opts,
				FrontendInputs: inputs,
				// Evaluate:       true, // TODO: maybe?
			})
			if err != nil {
				return nil, err
			}

			bkref, err := res.SingleRef()
			if err != nil {
				return nil, statusProxy.NiceError("build failed", err)
			}

			if bkref == nil {
				st = llb.Scratch()
			} else {
				st, err = bkref.ToState()
				if err != nil {
					return nil, err
				}
			}

			cfgBytes, found := res.Metadata[exptypes.ExporterImageConfigKey]
			if found {
				st, err = st.WithImageConfig(cfgBytes)
				if err != nil {
					return nil, fmt.Errorf("with image config: %w", err)
				}
			}

			return res, nil
		}

		_, err = b.runtime.Client.Build(ctx, kitdclient.SolveOpt{
			Session: []session.Attachable{
				b.runtime.authp,
			},
			AllowedEntitlements: allowed,
			LocalDirs:           b.localDirs,
		}, buildkitProduct, doBuild, statusProxy.Writer())
		if err != nil {
			return ib, statusProxy.NiceError("build failed", err)
		}

		wd, err := st.GetDir(ctx)
		if err != nil {
			return ib, fmt.Errorf("get dir: %w", err)
		}

		ib.fs = st
		ib.output = st
		ib.sourcePath = wd
		ib.needsInsecure = needsInsecure
		return ib, nil
	}

	return ib, fmt.Errorf("unsupported image type: %s", image.ToValue())
}

func (b *buildkitBuilder) buildInput(ctx context.Context, input bass.ImageBuildInput) (llb.State, string, bool, error) {
	var st llb.State
	var sourcePath string
	var needsInsecure bool

	var err error
	switch {
	case input.Thunk != nil:
		st, sourcePath, needsInsecure, err = b.thunkPathSt(ctx, *input.Thunk)
	case input.Host != nil:
		st, sourcePath, err = b.hostPathSt(ctx, *input.Host)
	case input.FS != nil:
		st, sourcePath, err = b.fsPathSt(ctx, *input.FS)
	default:
		err = fmt.Errorf("unknown build input: %s", input.ToValue())
	}
	if err != nil {
		return llb.State{}, "", false, fmt.Errorf("build input: %w", err)
	}

	return st, sourcePath, needsInsecure, nil
}

func (b *buildkitBuilder) initializeMount(ctx context.Context, source bass.ThunkMountSource, targetPath string) (llb.RunOption, string, bool, error) {
	switch {
	case source.ThunkPath != nil:
		st, sp, ni, err := b.thunkPathSt(ctx, *source.ThunkPath)
		if err != nil {
			return nil, "", false, fmt.Errorf("thunk llb: %w", err)
		}

		return llb.AddMount(targetPath, st, llb.SourcePath(sp)), sp, ni, nil

	case source.HostPath != nil:
		st, sp, err := b.hostPathSt(ctx, *source.HostPath)
		if err != nil {
			return nil, "", false, fmt.Errorf("thunk llb: %w", err)
		}

		return llb.AddMount(targetPath, st, llb.SourcePath(sp)), sp, false, nil

	case source.FSPath != nil:
		st, sp, err := b.fsPathSt(ctx, *source.FSPath)
		if err != nil {
			return nil, "", false, fmt.Errorf("thunk llb: %w", err)
		}

		return llb.AddMount(targetPath, st, llb.SourcePath(sp)), sp, false, nil

	case source.Cache != nil:
		return llb.AddMount(
			targetPath,
			llb.Scratch(),
			llb.AsPersistentCacheDir(source.Cache.ID, llb.CacheMountLocked),
			llb.SourcePath(source.Cache.Path.FilesystemPath().FromSlash()),
		), "", false, nil

	case source.Secret != nil:
		id := source.Secret.Name
		b.secrets[id] = source.Secret.Reveal()
		return llb.AddSecret(targetPath, llb.SecretID(id)), "", false, nil

	default:
		return nil, "", false, fmt.Errorf("unrecognized mount source: %s", source.ToValue())
	}
}

func (b *buildkitBuilder) thunkPathSt(ctx context.Context, source bass.ThunkPath) (llb.State, string, bool, error) {
	ib, err := b.llb(ctx, source.Thunk, getWorkdir)
	if err != nil {
		return llb.State{}, "", false, fmt.Errorf("thunk llb: %w", err)
	}

	return ib.output,
		filepath.Join(ib.sourcePath, source.Path.FilesystemPath().FromSlash()),
		ib.needsInsecure,
		nil
}

func (b *buildkitBuilder) hostPathSt(ctx context.Context, source bass.HostPath) (llb.State, string, error) {
	contextDir := source.ContextDir
	b.localDirs[contextDir] = source.ContextDir

	var excludes []string
	ignorePath := filepath.Join(contextDir, ".bassignore")
	ignore, err := os.Open(ignorePath)
	if err == nil {
		excludes, err = dockerignore.ReadAll(ignore)
		if err != nil {
			return llb.State{}, "", fmt.Errorf("parse %s: %w", ignorePath, err)
		}
	}

	sourcePath := source.Path.FilesystemPath().FromSlash()
	st := llb.Scratch().File(llb.Copy(
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
	))

	return st, sourcePath, nil
}

func (b *buildkitBuilder) fsPathSt(ctx context.Context, source bass.FSPath) (llb.State, string, error) {
	sourcePath := source.Path.FilesystemPath().FromSlash()

	if source.Path.File != nil {
		content, err := fs.ReadFile(source.FS, path.Clean(source.Path.Slash()))
		if err != nil {
			return llb.State{}, "", err
		}

		tree := llb.Scratch()

		filePath := path.Clean(source.Path.Slash())
		if strings.Contains(filePath, "/") {
			tree = tree.File(llb.Mkdir(path.Dir(filePath), 0755, llb.WithParents(true)))
		}

		return tree.File(llb.Mkfile(filePath, 0644, content)), sourcePath, nil
	} else {
		tree := llb.Scratch()

		err := fs.WalkDir(source.FS, path.Clean(source.Path.Slash()), func(walkPath string, d fs.DirEntry, err error) error {
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
				content, err := fs.ReadFile(source.FS, walkPath)
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
			return llb.State{}, "", fmt.Errorf("walk %s: %w", source, err)
		}

		return tree, sourcePath, nil
	}
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

func getWorkdir(st llb.ExecState, _ string) llb.State {
	return st.GetMount(workDir)
}
