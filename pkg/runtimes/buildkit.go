package runtimes

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/adrg/xdg"
	"github.com/pkg/errors"
	"github.com/zeebo/xxh3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/pkg/transfer/archive"
	"github.com/containerd/containerd/platforms"
	dockerconfig "github.com/docker/cli/cli/config"
	"github.com/docker/distribution/reference"
	"github.com/hashicorp/go-multierror"
	bkclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
	"github.com/moby/buildkit/frontend/dockerui"
	gwclient "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/session/secrets"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/morikuni/aec"
	"github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/tonistiigi/fsutil"
	fstypes "github.com/tonistiigi/fsutil/types"
	"github.com/tonistiigi/units"
	"github.com/vito/progrock"
	"go.uber.org/zap"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstls"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/runtimes/util"
	"github.com/vito/bass/pkg/runtimes/util/buildkitd"
	"github.com/vito/bass/pkg/zapctx"
)

const buildkitProduct = "bass"

const ociStoreName = "bass"

// OCI manifest annotation that specifies an image's tag
const ociTagAnnotation = "org.opencontainers.image.ref.name"

type BuildkitConfig struct {
	Oneshot      bool   `json:"oneshot,omitempty"`
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
	Platform ocispecs.Platform
	Inputs   map[string]llb.State

	client  *bkclient.Client
	gateway *RecordingGateway

	solveOpt bkclient.SolveOpt

	secrets  *secretStore
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

	if config.OCIStoreDir == "" {
		config.OCIStoreDir = filepath.Join(xdg.DataHome, "bass", "oci")
	}

	if config.CertsDir != "" {
		err := basstls.Init(config.CertsDir)
		if err != nil {
			return nil, fmt.Errorf("init tls depot: %w", err)
		}
	}

	client, err := dialBuildkit(ctx, config.Addr, config.Installation, config.CertsDir)
	if err != nil {
		return nil, fmt.Errorf("dial buildkit: %w", err)
	}

	workers, err := client.ListWorkers(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("list buildkit workers: %w", err)
	}

	var checkSame platforms.Matcher
	var platform ocispecs.Platform
	for _, w := range workers {
		if checkSame != nil && !checkSame.Match(w.Platforms[0]) {
			return nil, fmt.Errorf("TODO: workers have different platforms: %s != %s", w.Platforms[0], platform)
		}

		platform = w.Platforms[0]
		checkSame = platforms.Only(platform)
	}

	authp := authprovider.NewDockerAuthProvider(
		dockerconfig.LoadDefaultConfigFile(os.Stderr),
	)

	secrets := newSecretStore()

	ociStore, err := local.NewStore(config.OCIStoreDir)
	if err != nil {
		return nil, fmt.Errorf("create oci store: %w", err)
	}

	solveOpt := newSolveOpt(authp, secrets, ociStore)

	runtime := &Buildkit{
		Config: config,

		// TODO: report all supported platforms by workers instead
		Platform: platform,

		client: client,

		secrets:  secrets,
		ociStore: ociStore,
		solveOpt: solveOpt,
	}

	var gw gwclient.Client
	if config.Oneshot {
		gwCh := make(chan gwclient.Client, 1)
		gwErrCh := make(chan error, 1)
		go func() {
			statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
			defer statusProxy.Wait()

			_, err := client.Build(
				ctx,
				solveOpt,
				buildkitProduct,
				func(_ context.Context, gw gwclient.Client) (*gwclient.Result, error) {
					gwCh <- gw
					<-ctx.Done()
					return gwclient.NewResult(), nil
				},
				statusProxy.Writer(),
			)
			if err != nil {
				gwErrCh <- err
			}
		}()

		select {
		case gw = <-gwCh:
			runtime.gateway = &RecordingGateway{gw}
		case err := <-gwErrCh:
			return nil, fmt.Errorf("buildkit gateway: %w", err)
		}
	}

	return runtime, nil
}

func NewBuildkitFrontend(gw gwclient.Client, inputs map[string]llb.State, config BuildkitConfig) (*Buildkit, error) {
	if config.OCIStoreDir == "" {
		config.OCIStoreDir = filepath.Join(xdg.DataHome, "bass", "oci")
	}

	authp := authprovider.NewDockerAuthProvider(
		dockerconfig.LoadDefaultConfigFile(os.Stderr),
	)

	secrets := newSecretStore()

	ociStore, err := local.NewStore(config.OCIStoreDir)
	if err != nil {
		return nil, fmt.Errorf("create oci store: %w", err)
	}

	solveOpt := newSolveOpt(authp, secrets, ociStore)

	return &Buildkit{
		Config: config,
		Platform: ocispecs.Platform{
			OS:           runtime.GOOS,
			Architecture: runtime.GOARCH,
		},
		Inputs: inputs,

		gateway: &RecordingGateway{gw},

		secrets:  secrets,
		ociStore: ociStore,
		solveOpt: solveOpt,
	}, nil
}

func (buildkit *Buildkit) Client() (*bkclient.Client, error) {
	if buildkit.client == nil {
		return nil, fmt.Errorf("buildkit client unavailable")
	}

	return buildkit.client, nil
}

func (runtime *Buildkit) WithGateway(ctx context.Context, doBuild func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error)) error {
	if runtime.gateway != nil {
		_, err := doBuild(ctx, *runtime.gateway)
		return err
	}

	statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
	defer statusProxy.Wait()

	_, err := runtime.client.Build(
		ctx,
		runtime.solveOpt,
		buildkitProduct,
		func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
			return doBuild(ctx, RecordingGateway{gw})
		},
		statusProxy.Writer(),
	)
	if err != nil {
		return statusProxy.NiceError("resolve failed", err)
	}

	return nil
}

func (runtime *Buildkit) Resolve(ctx context.Context, imageRef bass.ImageRef) (bass.Thunk, error) {
	// track dependent services
	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()

	ctx, rec := progrock.WithGroup(ctx, "resolve "+imageRef.Thunk().String())
	defer rec.Complete()

	ref, err := ref(ctx, runtime, imageRef)
	if err != nil {
		// TODO: it might make sense to resolve an OCI archive ref to a digest too
		return bass.Thunk{}, fmt.Errorf("resolve ref %v: %w", imageRef, err)
	}

	// convert 'ubuntu' to 'docker.io/library/ubuntu:latest'
	normalized, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return bass.Thunk{}, fmt.Errorf("normalize ref: %w", err)
	}

	err = runtime.WithGateway(ctx, func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
		digest, _, err := gw.ResolveImageConfig(ctx, normalized.String(), llb.ResolveImageConfigOpt{
			Platform: &runtime.Platform,
		})
		if err != nil {
			return nil, err
		}

		imageRef.Digest = digest.String()

		return &gwclient.Result{}, nil
	})
	if err != nil {
		return bass.Thunk{}, fmt.Errorf("resolve image config: %w", err)
	}

	return imageRef.Thunk(), nil
}

func (runtime *Buildkit) Run(ctx context.Context, thunk bass.Thunk) error {
	ctx, rec := progrock.WithGroup(ctx, "run "+thunk.String())
	defer rec.Complete()

	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()
	_, err := runtime.build(
		ctx,
		thunk,
		nil, // exports
		func(ctx context.Context, gw gwclient.Client, ib IntermediateBuild) (*gwclient.Result, error) {
			return ib.ForRun(ctx, gw)
		},
		true, // inherit entrypoint/cmd
	)
	return err
}

func (runtime *Buildkit) Start(ctx context.Context, thunk bass.Thunk) (StartResult, error) {
	ctx, rec := progrock.WithGroup(ctx, "start "+thunk.String())
	defer rec.Complete()

	var res StartResult

	err := runtime.WithGateway(ctx, func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
		builder := runtime.NewBuilder(gw)

		var err error
		res, err = builder.Start(ctx, thunk)
		if err != nil {
			return nil, err
		}

		return gwclient.NewResult(), nil
	})
	if err != nil {
		return res, fmt.Errorf("start: %w", err)
	}

	return res, err
}

func (b *buildkitBuilder) Start(ctx context.Context, thunk bass.Thunk) (StartResult, error) {
	ctx, rec := progrock.WithGroup(ctx, "start "+thunk.String())
	defer rec.Complete()

	host := thunk.Name()

	health := newHealth(b.gw, b.platform, host, thunk.Ports)

	ib, err := b.Build(ctx, thunk, true)
	if err != nil {
		return StartResult{}, err
	}

	def, err := ib.FS.Marshal(ctx, llb.Platform(b.platform))
	if err != nil {
		return StartResult{}, err
	}

	ctx, stop := context.WithCancel(ctx)

	runs := bass.RunsFromContext(ctx)

	checked := make(chan error, 1)
	runs.Go(stop, func() error {
		res := health.Check(ctx)
		checked <- res
		return nil
	})

	exited := make(chan error, 1)
	runs.Go(stop, func() error {
		_, err := b.gw.Solve(ctx, gwclient.SolveRequest{
			Evaluate:   true,
			Definition: def.ToPB(),
		})
		exited <- err
		return err
	})

	select {
	case err := <-checked:
		if err != nil {
			return StartResult{}, fmt.Errorf("check error: %w", err)
		}

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
	ctx, rec := progrock.WithGroup(ctx, "read "+thunk.String())
	defer rec.Complete()

	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()

	_, err := runtime.build(
		ctx,
		thunk,
		nil,
		func(ctx context.Context, gw gwclient.Client, ib IntermediateBuild) (*gwclient.Result, error) {
			return ib.ReadStdout(ctx, gw, w)
		},
		true, // inherit entrypoint/cmd
	)
	return err
}

type marshalable interface {
	Marshal(ctx context.Context, co ...llb.ConstraintsOpt) (*llb.Definition, error)
}

func (runtime *Buildkit) Export(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	ctx, rec := progrock.WithGroup(ctx, "export "+thunk.String())
	defer rec.Complete()

	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()
	_, err := runtime.build(
		ctx,
		thunk,
		[]bkclient.ExportEntry{
			{
				Type: bkclient.ExporterOCI,
				Output: func(map[string]string) (io.WriteCloser, error) {
					return nopCloser{w}, nil
				},
			},
		},
		func(ctx context.Context, gw gwclient.Client, ib IntermediateBuild) (*gwclient.Result, error) {
			return ib.ForPublish(ctx, gw)
		},
		false, // do not inherit entrypoint/cmd
	)
	return err
}

func (runtime *Buildkit) Publish(ctx context.Context, ref bass.ImageRef, thunk bass.Thunk) (bass.ImageRef, error) {
	ctx, rec := progrock.WithGroup(ctx, "publish "+thunk.String())
	defer rec.Complete()

	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()

	addr, err := ref.Ref()
	if err != nil {
		return ref, err
	}

	res, err := runtime.build(
		ctx,
		thunk,
		[]bkclient.ExportEntry{
			{
				Type: bkclient.ExporterImage,
				Attrs: map[string]string{
					"name": addr,
					"push": "true",
				},
			},
		},
		func(ctx context.Context, gw gwclient.Client, ib IntermediateBuild) (*gwclient.Result, error) {
			return ib.ForPublish(ctx, gw)
		},
		false, // do not inherit entrypoint/cmd
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
	ctx, rec := progrock.WithGroup(ctx, "export path "+tp.String())
	defer rec.Complete()

	ctx, svcs := bass.TrackRuns(ctx)
	defer svcs.StopAndWait()

	thunk := tp.Thunk
	path := tp.Path

	var err error
	if path.FilesystemPath().IsDir() {
		_, err = runtime.build(
			ctx,
			thunk,
			[]bkclient.ExportEntry{
				{
					Type: bkclient.ExporterTar,
					Output: func(map[string]string) (io.WriteCloser, error) {
						return nopCloser{w}, nil
					},
				},
			},
			func(ctx context.Context, gw gwclient.Client, ib IntermediateBuild) (*gwclient.Result, error) {
				return ib.ForExportDir(ctx, gw, *path.Dir)
			},
			true, // inherit entryopint/cmd
		)
	} else {
		tw := tar.NewWriter(w)
		_, err = runtime.build(
			ctx,
			thunk,
			nil,
			func(ctx context.Context, gw gwclient.Client, ib IntermediateBuild) (*gwclient.Result, error) {
				return ib.ExportFile(ctx, gw, tw, *path.File)
			},
			true,
		)
	}
	return err
}

func (runtime *Buildkit) Prune(ctx context.Context, opts bass.PruneOpts) error {
	stderr := ioctx.StderrFromContext(ctx)
	tw := tabwriter.NewWriter(stderr, 2, 8, 2, ' ', 0)

	ch := make(chan bkclient.UsageInfo)
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

	kitdOpts := []bkclient.PruneOption{
		bkclient.WithKeepOpt(opts.KeepDuration, opts.KeepBytes),
	}

	if opts.All {
		kitdOpts = append(kitdOpts, bkclient.PruneAll)
	}

	client, err := runtime.Client()
	if err != nil {
		return err
	}

	err = client.Prune(ctx, ch, kitdOpts...)
	close(ch)
	<-printed
	if err != nil {
		return err
	}

	fmt.Fprintf(tw, "total: %.2f\n", units.Bytes(total))

	return tw.Flush()
}

func (runtime *Buildkit) Close() error {
	if runtime.client != nil {
		return runtime.client.Close()
	}
	return nil
}

func (runtime *Buildkit) build(
	ctx context.Context,
	thunk bass.Thunk,
	exports []bkclient.ExportEntry,
	cb func(context.Context, gwclient.Client, IntermediateBuild) (*gwclient.Result, error),
	forceExec bool,
	runOpts ...llb.RunOption,
) (*bkclient.SolveResponse, error) {
	// build llb definition using the remote gateway for image resolution
	var ib IntermediateBuild
	err := runtime.WithGateway(ctx, func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
		b := runtime.NewBuilder(gw)

		var err error
		ib, err = b.Build(ctx, thunk, forceExec, runOpts...)
		if err != nil {
			return nil, err
		}

		return &gwclient.Result{}, nil
	})
	if err != nil {
		return nil, err
	}

	doBuild := func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
		return cb(ctx, RecordingGateway{gw}, ib)
	}

	if len(exports) > 0 {
		solveOpt := runtime.solveOpt
		solveOpt.Exports = exports

		if client, err := runtime.Client(); err == nil {
			statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
			defer statusProxy.Wait()
			return client.Build(ctx, solveOpt, buildkitProduct, doBuild, statusProxy.Writer())
		}

		return nil, fmt.Errorf("gateway client does not support exporting")
	}

	err = runtime.WithGateway(ctx, doBuild)
	if err != nil {
		return nil, err
	}

	return &bkclient.SolveResponse{}, nil
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
	gw       gwclient.Client
	platform ocispecs.Platform

	host  string
	ports []bass.ThunkPort
}

func newHealth(gw gwclient.Client, platform ocispecs.Platform, host string, ports []bass.ThunkPort) *portHealthChecker {
	return &portHealthChecker{
		gw:       gw,
		platform: platform,
		host:     host,
		ports:    ports,
	}
}

func (d *portHealthChecker) Check(ctx context.Context) error {
	shimExe, err := shim(d.platform.Architecture)
	if err != nil {
		return err
	}

	shimRes, err := result(ctx, d.gw, shimExe)
	if err != nil {
		return err
	}

	scratchRes, err := result(ctx, d.gw, llb.Scratch())
	if err != nil {
		return err
	}

	container, err := d.gw.NewContainer(ctx, gwclient.NewContainerRequest{
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
		return err
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
		return err
	}

	exited := make(chan error, 1)
	go func() {
		exited <- proc.Wait()
	}()

	select {
	case err := <-exited:
		if err != nil {
			return err
		}

		return nil
	case <-ctx.Done():
		err := proc.Signal(cleanupCtx, syscall.SIGKILL)
		if err != nil {
			return fmt.Errorf("interrupt check: %w", err)
		}

		<-exited

		return ctx.Err()
	}
}

type buildkitBuilder struct {
	gw           gwclient.Client
	platform     ocispecs.Platform
	inputs       map[string]llb.State
	certsDir     string
	ociStore     content.Store
	secrets      *secretStore
	debug        bool
	disableCache bool
}

func (runtime *Buildkit) NewBuilder(client gwclient.Client) *buildkitBuilder {
	return NewBuilder(
		client,
		runtime.Platform,
		runtime.Inputs,
		runtime.Config.CertsDir,
		runtime.secrets,
		runtime.ociStore,
		runtime.Config.Debug,
		runtime.Config.DisableCache,
	)
}

func NewBuilder(
	client gwclient.Client,
	platform ocispecs.Platform,
	inputs map[string]llb.State,
	certsDir string,
	secrets *secretStore,
	ociStore content.Store,
	debug, disableCache bool,
) *buildkitBuilder {
	return &buildkitBuilder{
		gw:           client,
		platform:     platform,
		inputs:       inputs,
		certsDir:     certsDir,
		secrets:      secrets,
		ociStore:     ociStore,
		debug:        debug,
		disableCache: disableCache,
	}
}

func (b *buildkitBuilder) Build(
	ctx context.Context,
	thunk bass.Thunk,
	forceExec bool,
	extraOpts ...llb.RunOption,
) (IntermediateBuild, error) {
	ib, err := b.image(ctx, thunk.Image)
	if err != nil {
		return ib, err
	}

	thunkName, err := thunk.Hash()
	if err != nil {
		return ib, err
	}

	cmd, err := NewCommand(ctx, b, thunk)
	if err != nil {
		return ib, err
	}

	// propagate thunk's entrypoint to the child
	if len(thunk.Entrypoint) > 0 || thunk.ClearEntrypoint {
		ib.Config.Entrypoint = thunk.Entrypoint
	}

	// propagate thunk's default command
	if len(thunk.DefaultArgs) > 0 || thunk.ClearDefaultArgs {
		ib.Config.Cmd = thunk.DefaultArgs
	}

	if thunk.Labels != nil {
		ib.Config.Labels = map[string]string{}
		err := thunk.Labels.Each(func(k bass.Symbol, v bass.Value) error {
			var str string
			if err := v.Decode(&str); err != nil {
				return err
			}

			ib.Config.Labels[k.String()] = str
			return nil
		})
		if err != nil {
			return ib, fmt.Errorf("labels: %w", err)
		}
	}

	if len(thunk.Ports) > 0 {
		if ib.Config.ExposedPorts == nil {
			ib.Config.ExposedPorts = map[string]struct{}{}
		}
		for _, port := range thunk.Ports {
			ib.Config.ExposedPorts[fmt.Sprintf("%d/tcp", port.Port)] = struct{}{}
		}
	}

	useEntrypoint := thunk.UseEntrypoint
	if len(cmd.Args) == 0 {
		if forceExec {
			cmd.Args = ib.Config.Cmd
			useEntrypoint = true
		} else {
			// no command; just overriding config
			return ib, nil
		}
	}

	if useEntrypoint {
		cmd.Args = append(ib.Config.Entrypoint, cmd.Args...)
	}

	if len(cmd.Args) == 0 {
		return ib, fmt.Errorf("no command specified")
	}

	cmdPayload, err := bass.MarshalJSON(cmd)
	if err != nil {
		return ib, err
	}

	shimExe, err := shim(b.platform.Architecture)
	if err != nil {
		return ib, err
	}

	runOpt := []llb.RunOption{
		llb.WithCustomName(thunk.Cmdline()),
		llb.AddMount("/tmp", llb.Scratch(), llb.Tmpfs()),
		llb.AddMount("/dev/shm", llb.Scratch(), llb.Tmpfs()),
		llb.AddMount(ioDir, llb.Scratch().File(
			llb.Mkfile("in", 0600, cmdPayload),
			llb.WithCustomNamef("[hide] mount command json for %s", thunk.String()),
		)),
		llb.AddMount(shimExePath, shimExe, llb.SourcePath("run")),
		llb.With(llb.Dir(workDir)),
		llb.AddEnv("_BASS_OUTPUT", outputFile),
		llb.Args([]string{shimExePath, "run", inputFile}),
	}

	if len(thunk.Ports) > 0 {
		// NB: only set the thunk name as the hostname for services. otherwise
		// we'll bust caches in cases where e.g. globs would otherwise prevent it.
		runOpt = append(runOpt, llb.Hostname(thunkName))
	} else {
		runOpt = append(runOpt, llb.Hostname("thunk"))
	}

	if b.certsDir != "" {
		rootCA, err := os.ReadFile(basstls.CACert(b.certsDir))
		if err != nil {
			return ib, err
		}

		runOpt = append(runOpt,
			llb.AddMount(caFile, llb.Scratch().File(
				llb.Mkfile("ca.crt", 0600, rootCA),
				llb.WithCustomName("[hide] mount bass ca"),
			), llb.SourcePath("ca.crt")))
	}

	if thunk.TLS != nil {
		if b.certsDir == "" {
			return ib, fmt.Errorf("TLS not configured")
		}

		crt, key, err := basstls.Generate(b.certsDir, thunkName)
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

	if b.debug {
		runOpt = append(runOpt, llb.AddEnv("_BASS_DEBUG", "1"))
	}

	for _, env := range cmd.SecretEnv {
		id := b.secrets.PutSecret(env.Secret.Reveal())
		runOpt = append(runOpt, llb.AddSecret(env.Name, llb.SecretID(id), llb.SecretAsEnv(true)))
	}

	if thunk.Insecure {
		ib.NeedsInsecure = true

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
			ib.OutputSourcePath = sp
		}

		if ni {
			ib.NeedsInsecure = true
		}

		runOpt = append(runOpt, mountOpt)
	}

	if !remountedWorkdir {
		if ib.OutputSourcePath != "" {
			// NB: could just call SourcePath with "", but this is to ensure there's
			// code coverage
			runOpt = append(runOpt, llb.AddMount(workDir, ib.Output, llb.SourcePath(ib.OutputSourcePath)))
		} else {
			runOpt = append(runOpt, llb.AddMount(workDir, ib.Output))
		}
	}

	if b.disableCache {
		runOpt = append(runOpt, llb.IgnoreCache)
	}

	runOpt = append(runOpt, extraOpts...)

	execSt := ib.FS.Run(runOpt...)
	ib.Exec = execSt
	ib.Output = execSt.GetMount(workDir)
	ib.FS = execSt.Root()

	return ib, nil
}

func shim(arch string) (llb.State, error) {
	shimExe, found := allShims["exe."+arch]
	if !found {
		return llb.State{}, fmt.Errorf("no shim found for %s", arch)
	}

	return llb.Scratch().File(
		llb.Mkfile("/run", 0755, shimExe),
		llb.WithCustomName("[hide] load bass shim"),
	), nil
}

func ref(ctx context.Context, starter Starter, imageRef bass.ImageRef) (string, error) {
	if imageRef.Repository.Addr != nil {
		addr := imageRef.Repository.Addr

		result, err := starter.Start(ctx, addr.Thunk)
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

type IntermediateBuild struct {
	FS     llb.State
	Exec   llb.ExecState
	Output llb.State

	OutputSourcePath string
	NeedsInsecure    bool

	Platform ocispecs.Platform
	Config   ocispecs.ImageConfig
}

func (ib IntermediateBuild) WithImageConfig(config []byte) (IntermediateBuild, error) {
	var img ocispecs.Image
	if err := json.Unmarshal(config, &img); err != nil {
		return ib, err
	}

	ib.Config = img.Config

	for _, env := range img.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts[0]) > 0 {
			var v string
			if len(parts) > 1 {
				v = parts[1]
			}
			ib.FS = ib.FS.AddEnv(parts[0], v)
		}
	}

	ib.FS = ib.FS.Dir(img.Config.WorkingDir)
	if img.Architecture != "" && img.OS != "" {
		ib.FS = ib.FS.Platform(ocispecs.Platform{
			OS:           img.OS,
			Architecture: img.Architecture,
			Variant:      img.Variant,
		})
	}

	return ib, nil
}

func (ib IntermediateBuild) ForPublish(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
	def, err := ib.FS.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	res, err := gw.Solve(ctx, gwclient.SolveRequest{
		Evaluate:   true,
		Definition: def.ToPB(),
	})
	if err != nil {
		return nil, err
	}

	cfgBytes, err := json.Marshal(ocispecs.Image{
		Architecture: ib.Platform.Architecture,
		OS:           ib.Platform.OS,
		OSVersion:    ib.Platform.OSVersion,
		OSFeatures:   ib.Platform.OSFeatures,
		Config:       ib.Config,
	})
	if err != nil {
		return nil, err
	}
	res.AddMeta(exptypes.ExporterImageConfigKey, cfgBytes)

	return res, nil
}

func (ib IntermediateBuild) ForRun(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
	def, err := ib.Exec.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	return gw.Solve(ctx, gwclient.SolveRequest{
		Evaluate:   true,
		Definition: def.ToPB(),
	})
}

func (ib IntermediateBuild) ReadStdout(ctx context.Context, gw gwclient.Client, w io.Writer) (*gwclient.Result, error) {
	def, err := ib.Exec.GetMount(ioDir).Marshal(ctx)
	if err != nil {
		return nil, err
	}

	res, err := gw.Solve(ctx, gwclient.SolveRequest{
		Evaluate:   true,
		Definition: def.ToPB(),
	})
	if err != nil {
		return nil, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return nil, err
	}

	fs := util.NewRefFS(ctx, ref)

	f, err := fs.Open(path.Base(outputFile))
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(w, f); err != nil {
		return nil, err
	}

	if err := f.Close(); err != nil {
		return nil, err
	}

	return res, nil
}

func (ib IntermediateBuild) ForExportDir(ctx context.Context, gw gwclient.Client, fsp bass.DirPath) (*gwclient.Result, error) {
	copyOpt := &llb.CopyInfo{
		IncludePatterns: fsp.Includes(),
		ExcludePatterns: fsp.Excludes(),
		AllowWildcard:   true,
	}

	if fsp.IsDir() {
		copyOpt.CopyDirContentsOnly = true
	}

	st := llb.Scratch().File(
		llb.Copy(
			ib.Output,
			filepath.Join(ib.OutputSourcePath, fsp.FromSlash()),
			".",
			copyOpt,
		),
		llb.WithCustomNamef("copy %s", fsp.Slash()),
	)

	def, err := st.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	return gw.Solve(ctx, gwclient.SolveRequest{
		Evaluate:   true,
		Definition: def.ToPB(),
	})
}

func (ib IntermediateBuild) ExportFile(ctx context.Context, gw gwclient.Client, tw *tar.Writer, fsp bass.FilePath) (*gwclient.Result, error) {
	def, err := ib.Output.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	res, err := gw.Solve(ctx, gwclient.SolveRequest{
		Evaluate:   true,
		Definition: def.ToPB(),
	})
	if err != nil {
		return nil, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(ib.OutputSourcePath, fsp.FromSlash())

	stat, err := ref.StatFile(ctx, gwclient.StatRequest{
		Path: filePath,
	})
	if err != nil {
		return nil, err
	}

	err = tw.WriteHeader(&tar.Header{
		Name:     fsp.FromSlash(),
		Mode:     int64(stat.Mode),
		Uid:      int(stat.Uid),
		Gid:      int(stat.Gid),
		Size:     int64(stat.Size_),
		ModTime:  time.Unix(stat.ModTime/int64(time.Second), stat.ModTime%int64(time.Second)),
		Linkname: stat.Linkname,
		Devmajor: stat.Devmajor,
		Devminor: stat.Devmajor,
	})
	if err != nil {
		return nil, fmt.Errorf("write tar header: %w", err)
	}

	fs := util.NewRefFS(ctx, ref)

	f, err := fs.Open(filePath)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 1*1024*1024)
	if _, err := io.CopyBuffer(tw, f, buf); err != nil {
		return nil, err
	}

	if err := f.Close(); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return res, nil
}

func (b *buildkitBuilder) image(ctx context.Context, image *bass.ThunkImage) (IntermediateBuild, error) {
	ib := IntermediateBuild{
		Platform: b.platform,
	}

	switch {
	case image == nil:
		// TODO: test; how is this possible?
		ib.FS = llb.Scratch()
		ib.Output = llb.Scratch()
		return ib, nil

	case image.Ref != nil:
		ref, err := ref(ctx, b, *image.Ref)
		if err != nil {
			return ib, err
		}

		r, err := reference.ParseNormalizedNamed(ref)
		if err == nil {
			r = reference.TagNameOnly(r)
			ref = r.String()
		}

		dgst, config, err := b.gw.ResolveImageConfig(ctx, ref, llb.ResolveImageConfigOpt{
			ResolverType: llb.ResolverTypeRegistry,
			Platform:     &b.platform,
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

		ib.FS = llb.Image(ref, llb.Platform(b.platform))

		ib, err = ib.WithImageConfig(config)
		if err != nil {
			return ib, fmt.Errorf("image archive with image config: %w", err)
		}

		ib.Output = llb.Scratch()
		return ib, nil

	case image.Thunk != nil:
		return b.Build(ctx, *image.Thunk, false)

	case image.Archive != nil:
		cachedIb, found := ociCache.Get(ctx, image.Archive)
		if found {
			return cachedIb, nil
		}

		file := image.Archive.File

		rc, err := file.ToReadable().Open(ctx)
		if err != nil {
			return ib, fmt.Errorf("image archive file: %w", err)
		}

		defer rc.Close()

		var desc ocispecs.Descriptor
		err = cli.Step(ctx, fmt.Sprintf("import %s", file.ToValue()), func(ctx context.Context, rec *progrock.VertexRecorder) error {
			desc, err = archive.NewImageImportStream(rc, "").Import(ctx, b.ociStore)
			return err
		})
		if err != nil {
			return ib, fmt.Errorf("image archive import: %w", err)
		}

		manifestDesc, err := resolveIndex(ctx, b.ociStore, desc, b.platform, image.Archive.Tag)
		if err != nil {
			return ib, fmt.Errorf("image archive resolve index: %w", err)
		}

		manifestBlob, err := content.ReadBlob(ctx, b.ociStore, *manifestDesc)
		if err != nil {
			return ib, fmt.Errorf("image archive read manifest blob: %w", err)
		}

		// NB: the repository portion of this ref doesn't actually matter, but it's
		// pleasant to see something recognizable.
		dummyRepo := path.Join("load", file.ToPath().Name())

		st := llb.OCILayout(
			fmt.Sprintf("%s@%s", dummyRepo, manifestDesc.Digest),
			llb.OCIStore("", ociStoreName),
			llb.Platform(b.platform),
		)

		var m ocispecs.Manifest
		err = json.Unmarshal(manifestBlob, &m)
		if err != nil {
			return ib, fmt.Errorf("image archive unmarshal manifest: %w", err)
		}

		configBlob, err := content.ReadBlob(ctx, b.ociStore, m.Config)
		if err != nil {
			return ib, fmt.Errorf("image archive read blob: %w", err)
		}

		ib.FS = st

		ib, err = ib.WithImageConfig(configBlob)
		if err != nil {
			return ib, fmt.Errorf("image archive with image config: %w", err)
		}

		ib.Output = llb.Scratch()

		ociCache.Put(ctx, image.Archive, ib)

		return ib, nil
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

		if dockerfile != nil {
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

		inputs := map[string]*pb.Definition{
			dockerui.DefaultLocalNameContext:    ctxDef.ToPB(),
			dockerui.DefaultLocalNameDockerfile: ctxDef.ToPB(),
		}

		if b.certsDir != "" {
			certDef, err := llb.Local(b.certsDir,
				llb.IncludePatterns(basstls.CAFiles),
				llb.Differ(llb.DiffMetadata, false)).
				Marshal(ctx, llb.Platform(b.platform))
			if err != nil {
				return ib, fmt.Errorf("bass tls def: %w", err)
			}

			inputs["bass-tls"] = certDef.ToPB()
		}

		statusProxy := forwardStatus(progrock.RecorderFromContext(ctx))
		defer statusProxy.Wait()

		ctx, rec := progrock.WithGroup(ctx, "docker build "+contextDir.ToValue().String())
		defer rec.Complete()

		res, err := b.gw.Solve(ctx, gwclient.SolveRequest{
			Frontend:       "dockerfile.v0",
			FrontendOpt:    opts,
			FrontendInputs: inputs,
		})
		if err != nil {
			return ib, err
		}

		bkref, err := res.SingleRef()
		if err != nil {
			return ib, fmt.Errorf("single ref: %w", err)
		}

		var st llb.State
		if bkref == nil {
			st = llb.Scratch()
		} else {
			st, err = bkref.ToState()
			if err != nil {
				return ib, err
			}
		}

		ib.FS = st

		cfgBytes, found := res.Metadata[exptypes.ExporterImageConfigKey]
		if found {
			ib, err = ib.WithImageConfig(cfgBytes)
			if err != nil {
				return ib, fmt.Errorf("with image config: %w", err)
			}
		}

		ib.Output = ib.FS
		ib.NeedsInsecure = needsInsecure
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
		var mode llb.CacheMountSharingMode
		switch source.Cache.ConcurrencyMode {
		case bass.ConcurrencyModeShared:
			mode = llb.CacheMountShared
		case bass.ConcurrencyModePrivate:
			mode = llb.CacheMountPrivate
		case bass.ConcurrencyModeLocked:
			mode = llb.CacheMountLocked
		}

		return llb.AddMount(
			targetPath,
			llb.Scratch(),
			llb.AsPersistentCacheDir(source.Cache.ID, mode),
			llb.SourcePath(source.Cache.Path.FilesystemPath().FromSlash()),
		), "", false, nil

	case source.Secret != nil:
		id := b.secrets.PutSecret(source.Secret.Reveal())
		return llb.AddSecret(targetPath, llb.SecretID(id)), "", false, nil

	default:
		return nil, "", false, fmt.Errorf("unrecognized mount source: %s", source.ToValue())
	}
}

func (b *buildkitBuilder) thunkPathSt(ctx context.Context, source bass.ThunkPath) (llb.State, string, bool, error) {
	ib, err := b.Build(ctx, source.Thunk, true)
	if err != nil {
		return llb.State{}, "", false, fmt.Errorf("thunk llb: %w", err)
	}

	include := source.Includes()
	exclude := source.Excludes()

	var st llb.State
	var sourcePath string
	if len(include) > 0 || len(exclude) > 0 {
		st = llb.Scratch().File(
			llb.Copy(ib.Output, ib.OutputSourcePath, ".", &llb.CopyInfo{
				IncludePatterns:     include,
				ExcludePatterns:     exclude,
				CopyDirContentsOnly: true,
				AllowWildcard:       true,
			}),
		)

		sourcePath = source.Path.FilesystemPath().FromSlash()
	} else {
		st = ib.Output
		sourcePath = filepath.Join(ib.OutputSourcePath, source.Path.FilesystemPath().FromSlash())
	}

	return st, sourcePath, ib.NeedsInsecure, nil
}

func (b *buildkitBuilder) hostPathSt(ctx context.Context, source bass.HostPath) (llb.State, string, error) {
	localName := source.ContextDir

	sourcePath := source.Path.FilesystemPath().FromSlash()
	if input, found := b.inputs[localName]; found {
		return input, sourcePath, nil
	}

	include := source.Includes()
	exclude := source.Excludes()

	ignorePath := bass.HostPath{
		ContextDir: localName,
		Path:       bass.ParseFileOrDirPath(".bassignore"),
	}

	ignore, err := ignorePath.Open(ctx)
	if err == nil {
		ignore, err := dockerignore.ReadAll(ignore)
		if err != nil {
			return llb.State{}, "", fmt.Errorf("parse %s: %w", ignorePath, err)
		}

		exclude = append(exclude, ignore...)
	}

	st := llb.Scratch().File(llb.Copy(
		llb.Local(
			localName,
			llb.IncludePatterns(include),
			llb.ExcludePatterns(exclude),
			llb.Differ(llb.DiffMetadata, false),
			llb.WithCustomNamef("upload %s", source),

			// synchronize concurrent filesyncs for the same path
			llb.SharedKeyHint(source.Hash()),

			// make the LLB stable so we can test invariants like:
			//
			//   workdir == directory(".")
			llb.LocalUniqueID(source.Hash()),
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

func (proxy *statusProxy) proxy(rec *progrock.Recorder, statuses chan *bkclient.SolveStatus) {
	for {
		status, ok := <-statuses
		if !ok {
			break
		}

		update := bk2progrock(status)
		proxy.prog.WriteStatus(update)
		rec.Record(update)
	}
}

func (proxy *statusProxy) Writer() chan *bkclient.SolveStatus {
	statuses := make(chan *bkclient.SolveStatus)

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

func dialBuildkit(ctx context.Context, addr string, installation string, certsDir string) (*bkclient.Client, error) {
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

	client, err := bkclient.New(context.TODO(), addr)
	if err != nil {
		errs = multierror.Append(errs, err)
		return nil, errs
	}

	return client, nil
}

type AnyDirSource struct{}

func (AnyDirSource) LookupDir(name string) (filesync.SyncedDir, bool) {
	return filesync.SyncedDir{
		Dir: name,
		Map: func(p string, st *fstypes.Stat) fsutil.MapResult {
			st.Uid = 0
			st.Gid = 0
			return fsutil.MapResultKeep
		},
	}, true
}

func resolveIndex(ctx context.Context, store content.Store, desc ocispecs.Descriptor, platform ocispecs.Platform, tag string) (*ocispecs.Descriptor, error) {
	if desc.MediaType != ocispecs.MediaTypeImageIndex {
		return nil, fmt.Errorf("expected index, got %s", desc.MediaType)
	}

	indexBlob, err := content.ReadBlob(ctx, store, desc)
	if err != nil {
		return nil, fmt.Errorf("read index blob: %w", err)
	}

	var idx ocispecs.Index
	err = json.Unmarshal(indexBlob, &idx)
	if err != nil {
		return nil, fmt.Errorf("unmarshal index: %w", err)
	}

	matcher := platforms.Only(platform)

	for _, m := range idx.Manifests {
		if m.Platform != nil {
			if !matcher.Match(*m.Platform) {
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

		switch m.MediaType {
		case ocispecs.MediaTypeImageManifest, // OCI
			images.MediaTypeDockerSchema2Manifest: // Docker
			return &m, nil

		case ocispecs.MediaTypeImageIndex, // OCI
			images.MediaTypeDockerSchema2ManifestList: // Docker
			return resolveIndex(ctx, store, m, platform, tag)

		default:
			return nil, fmt.Errorf("expected manifest or index, got %s", m.MediaType)
		}
	}

	return nil, fmt.Errorf("no manifest for platform %s and tag %s", platforms.Format(platform), tag)
}

type secretStore struct {
	digest2secret map[string][]byte
	sync.Mutex
}

func newSecretStore() *secretStore {
	return &secretStore{
		digest2secret: make(map[string][]byte),
	}
}

func (s *secretStore) PutSecret(value []byte) string {
	s.Lock()
	defer s.Unlock()
	// generate a digest for the secret

	digest := sha256.Sum256(value)
	digestStr := hex.EncodeToString(digest[:])
	s.digest2secret[digestStr] = value

	return digestStr
}

func (s *secretStore) GetSecret(ctx context.Context, digest string) ([]byte, error) {
	s.Lock()
	defer s.Unlock()

	v, ok := s.digest2secret[digest]
	if !ok {
		return nil, errors.WithStack(secrets.ErrNotFound)
	}

	return v, nil
}

func newSolveOpt(
	authp session.Attachable,
	secrets *secretStore,
	ociStore content.Store,
) bkclient.SolveOpt {
	return bkclient.SolveOpt{
		AllowedEntitlements: []entitlements.Entitlement{
			entitlements.EntitlementSecurityInsecure,
		},
		Session: []session.Attachable{
			authp,
			secretsprovider.NewSecretProvider(secrets),
			filesync.NewFSSyncProvider(AnyDirSource{}),
		},
		OCIStores: map[string]content.Store{
			ociStoreName: ociStore,
		},
	}
}

type protoCache[T any] struct {
	cache map[uint64]T
	l     sync.Mutex
}

func newProtoCache[T any]() *protoCache[T] {
	return &protoCache[T]{
		cache: make(map[uint64]T),
	}
}

func (cache *protoCache[T]) Get(ctx context.Context, key bass.ProtoMarshaler) (T, bool) {
	cache.l.Lock()
	defer cache.l.Unlock()

	hash, err := hashProtoMessage(key)
	if err != nil {
		zapctx.FromContext(ctx).Error("oci archive marshal failed", zap.Error(err))
	}

	val, found := cache.cache[hash]
	return val, found
}

func (cache *protoCache[T]) Put(ctx context.Context, key bass.ProtoMarshaler, val T) {
	cache.l.Lock()
	defer cache.l.Unlock()

	hash, err := hashProtoMessage(key)
	if err != nil {
		zapctx.FromContext(ctx).Error("oci archive marshal failed", zap.Error(err))
	}

	cache.cache[hash] = val
}

// ociCache stores a cache of intermediate build results for OCI archive
// imports, since it can take quite a while to import them to the local store.
var ociCache = newProtoCache[IntermediateBuild]()

func hashProtoMessage(val bass.ProtoMarshaler) (uint64, error) {
	msg, err := val.MarshalProto()
	if err != nil {
		return 0, err
	}
	bytes, err := proto.Marshal(msg)
	if err != nil {
		return 0, err
	}
	return xxh3.Hash(bytes), nil
}

func bk2progrock(event *bkclient.SolveStatus) *progrock.StatusUpdate {
	var status progrock.StatusUpdate
	for _, v := range event.Vertexes {
		vtx := &progrock.Vertex{
			Id:     v.Digest.String(),
			Name:   v.Name,
			Cached: v.Cached,
		}
		if strings.Contains(v.Name, "[hide]") {
			vtx.Internal = true
		}
		for _, input := range v.Inputs {
			vtx.Inputs = append(vtx.Inputs, input.String())
		}
		if v.Started != nil {
			vtx.Started = timestamppb.New(*v.Started)
		}
		if v.Completed != nil {
			vtx.Completed = timestamppb.New(*v.Completed)
		}
		if v.Error != "" {
			if strings.HasSuffix(v.Error, context.Canceled.Error()) {
				vtx.Canceled = true
			} else {
				err := v.Error
				vtx.Error = &err
			}
		}
		// NB: ProgressGroup is ignored. By the time we see it here it's too late;
		// the only recorder available is the one established at gateway open time,
		// which might be the parent of the ProgressGroup.
		//
		// Bass doesn't set ProgressGroup itself, but they might be present in a
		// frontend build result (e.g. Dockerfile). Those are handled by the
		// RecordingGateway at Solve() time instead.
		// if v.ProgressGroup != nil {
		// }
		status.Vertexes = append(status.Vertexes, vtx)
	}

	for _, s := range event.Statuses {
		task := &progrock.VertexTask{
			Vertex:  s.Vertex.String(),
			Name:    s.ID, // remap
			Total:   s.Total,
			Current: s.Current,
		}
		if s.Started != nil {
			task.Started = timestamppb.New(*s.Started)
		}
		if s.Completed != nil {
			task.Completed = timestamppb.New(*s.Completed)
		}
		status.Tasks = append(status.Tasks, task)
	}

	for _, s := range event.Logs {
		status.Logs = append(status.Logs, &progrock.VertexLog{
			Vertex:    s.Vertex.String(),
			Stream:    progrock.LogStream(s.Stream),
			Data:      s.Data,
			Timestamp: timestamppb.New(s.Timestamp),
		})
	}

	return &status
}

type RecordingGateway struct {
	gwclient.Client
}

func (g RecordingGateway) ResolveImageConfig(ctx context.Context, ref string, opt llb.ResolveImageConfigOpt) (digest.Digest, []byte, error) {
	rec := progrock.RecorderFromContext(ctx)

	// HACK(vito): this is how Buildkit determines the vertex digest for
	// ResolveImageConfig. Keep this in sync with Buildkit until a better way to
	// do this arrives. It hasn't changed in 5 years, surely it won't soon,
	// right?
	id := ref
	if platform := opt.Platform; platform == nil {
		id += platforms.Format(platforms.DefaultSpec())
	} else {
		id += platforms.Format(*platform)
	}
	rec.Join(digest.FromString(id))

	return g.Client.ResolveImageConfig(ctx, ref, opt)
}

func (g RecordingGateway) Solve(ctx context.Context, opts gwclient.SolveRequest) (*gwclient.Result, error) {
	rec := progrock.RecorderFromContext(ctx)

	if opts.Definition != nil {
		g.recordVertexes(rec, opts.Definition)
	}

	for _, input := range opts.FrontendInputs {
		g.recordVertexes(rec, input)
	}

	res, err := g.Client.Solve(ctx, opts)
	if err != nil {
		return nil, err
	}

	// NB: when building with a frontend, e.g. a Dockerfile, record its outputs
	// as a member of the group too. this way you can put a full Dockerfile build
	// into a subgroup. without it, you would only see the vertexes for the
	// Dockerfile's inputs in the subgroup.
	if opts.Frontend != "" {
		bkref, err := res.SingleRef()
		if err != nil {
			return nil, err
		}

		var st llb.State
		if bkref == nil {
			st = llb.Scratch()
		} else {
			st, err = bkref.ToState()
			if err != nil {
				return nil, err
			}
		}

		def, err := st.Marshal(ctx)
		if err != nil {
			return nil, err
		}

		g.recordVertexes(rec, def.ToPB())
	}

	return res, nil
}

func (g RecordingGateway) recordVertexes(recorder *progrock.Recorder, def *pb.Definition) {
	dgsts := []digest.Digest{}
	for dgst, meta := range def.Metadata {
		if meta.ProgressGroup != nil {
			recorder.WithGroup(meta.ProgressGroup.Name).Join(dgst)
		} else {
			dgsts = append(dgsts, dgst)
		}
	}

	recorder.Join(dgsts...)
}
