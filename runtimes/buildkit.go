package runtimes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/adrg/xdg"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/mitchellh/go-homedir"
	"github.com/moby/buildkit/client"
	kitdclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
	gwclient "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/morikuni/aec"
	"github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/tonistiigi/units"
	"github.com/vito/bass"
	"github.com/vito/bass/ioctx"
	"github.com/vito/progrock"
	"github.com/vito/progrock/graph"
	bolt "go.etcd.io/bbolt"

	_ "embed"
)

const buildkitProduct = "bass"

type BuildkitConfig struct {
	BuildkitAddr string `json:"buildkit_addr,omitempty"`
	Data         string `json:"data,omitempty"`
	DisableCache bool   `json:"disable_cache,omitempty"`
}

func (config BuildkitConfig) ResponseDir(id string) string {
	return filepath.Join(config.Data, id)
}

func (config BuildkitConfig) ResponsePath(id string) string {
	return filepath.Join(config.ResponseDir(id), filepath.Base(outputFile))
}

func (config BuildkitConfig) Cleanup(id string) error {
	return os.RemoveAll(config.ResponseDir(id))
}

var _ bass.Runtime = &Buildkit{}

//go:embed shim/main.go
var shim []byte

const BuildkitName = "buildkit"

const runExe = "/bass/run"
const workDir = "/bass/work"
const ioDir = "/bass/io"
const inputFile = "/bass/io/in"
const outputFile = "/bass/io/out"

const digestBucket = "_digests"
const configBucket = "_configs"

func init() {
	RegisterRuntime(BuildkitName, NewBuildkit)
}

type Buildkit struct {
	Config   BuildkitConfig
	Client   *kitdclient.Client
	Platform ocispecs.Platform
}

func NewBuildkit(_ bass.RuntimePool, cfg *bass.Scope) (bass.Runtime, error) {
	var config BuildkitConfig
	if cfg != nil {
		if err := cfg.Decode(&config); err != nil {
			return nil, fmt.Errorf("docker runtime config: %w", err)
		}
	}

	dataDir := config.Data
	if dataDir == "" {
		dataDir = filepath.Join(xdg.CacheHome, "bass")
	}

	dataRoot, err := homedir.Expand(dataDir)
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	config.Data = dataRoot

	err = os.MkdirAll(dataRoot, 0700)
	if err != nil {
		return nil, err
	}

	client, err := dialBuildkit(config.BuildkitAddr)
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
	}, nil
}

func dialBuildkit(addr string) (*kitdclient.Client, error) {
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

	return kitdclient.New(context.TODO(), addr)
}

func (runtime *Buildkit) Resolve(ctx context.Context, imageRef bass.ThunkImageRef) (bass.ThunkImageRef, error) {
	// convert 'ubuntu' to 'docker.io/library/ubuntu:latest'
	normalized, err := reference.ParseNormalizedNamed(imageRef.Ref())
	if err != nil {
		return bass.ThunkImageRef{}, fmt.Errorf("normalize ref: %w", err)
	}

	_, err = runtime.Client.Build(ctx, kitdclient.SolveOpt{
		Session: []session.Attachable{
			authprovider.NewDockerAuthProvider(os.Stderr),
		},
	}, buildkitProduct, func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
		resolver := runtime.cacheResolver(gw)

		digest, _, err := resolver.ResolveImageConfig(ctx, normalized.String(), llb.ResolveImageConfigOpt{
			Platform: &runtime.Platform,
		})
		if err != nil {
			return nil, fmt.Errorf("resolve: %w", err)
		}

		imageRef.Digest = digest.String()

		return &gwclient.Result{}, nil
	}, forwardStatus(progrock.RecorderFromContext(ctx)))
	if err != nil {
		return bass.ThunkImageRef{}, fmt.Errorf("solve: %w", err)
	}

	return imageRef, nil
}

func (runtime *Buildkit) Run(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	id, err := thunk.SHA256()
	if err != nil {
		return err
	}

	err = runtime.build(
		ctx,
		thunk,
		w != io.Discard,
		func(st llb.ExecState) marshalable { return st.GetMount(ioDir) },
		kitdclient.ExportEntry{
			Type:      kitdclient.ExporterLocal,
			OutputDir: runtime.Config.ResponseDir(id),
		},
	)
	if err != nil {
		return err
	}

	response, err := os.Open(runtime.Config.ResponsePath(id))
	if err == nil {
		defer response.Close()

		_, err = io.Copy(w, response)
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}
	}

	return nil
}

func (runtime *Buildkit) Load(ctx context.Context, thunk bass.Thunk) (*bass.Scope, error) {
	// TODO: run thunk, parse response stream as bindings mapped to paths for
	// constructing thunks inheriting from the initial thunk
	return nil, nil
}

type marshalable interface {
	Marshal(ctx context.Context, co ...llb.ConstraintsOpt) (*llb.Definition, error)
}

func (runtime *Buildkit) Export(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	return runtime.build(
		ctx,
		thunk,
		false,
		func(st llb.ExecState) marshalable { return st },
		kitdclient.ExportEntry{
			Type: kitdclient.ExporterOCI,
			Output: func(map[string]string) (io.WriteCloser, error) {
				return nopCloser{w}, nil
			},
		},
	)
}

func (runtime *Buildkit) ExportPath(ctx context.Context, w io.Writer, tp bass.ThunkPath) error {
	thunk := tp.Thunk
	path := tp.Path

	return runtime.build(
		ctx,
		thunk,
		false,
		func(st llb.ExecState) marshalable {
			copyOpt := &llb.CopyInfo{}
			if path.FilesystemPath().IsDir() {
				copyOpt.CopyDirContentsOnly = true
			}

			return llb.Scratch().File(
				llb.Copy(st.GetMount(workDir), path.String(), ".", copyOpt),
			)
		},
		kitdclient.ExportEntry{
			Type: kitdclient.ExporterTar,
			Output: func(map[string]string) (io.WriteCloser, error) {
				return nopCloser{w}, nil
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

func (runtime *Buildkit) build(ctx context.Context, thunk bass.Thunk, captureStdout bool, transform func(llb.ExecState) marshalable, exports ...kitdclient.ExportEntry) error {
	var def *llb.Definition
	var secrets map[string][]byte
	var localDirs map[string]string
	var allowed []entitlements.Entitlement

	// build llb definition using the remote gateway for image resolution
	_, err := runtime.Client.Build(ctx, kitdclient.SolveOpt{
		Session: []session.Attachable{
			authprovider.NewDockerAuthProvider(os.Stderr),
		},
	}, buildkitProduct, func(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
		b := runtime.newBuilder(runtime.cacheResolver(gw))

		st, needsInsecure, err := b.llb(ctx, thunk, captureStdout)
		if err != nil {
			return nil, err
		}

		if needsInsecure {
			allowed = append(allowed, entitlements.EntitlementSecurityInsecure)
		}

		localDirs = b.localDirs
		secrets = b.secrets

		def, err = transform(st).Marshal(ctx)
		if err != nil {
			return nil, err
		}

		return &gwclient.Result{}, nil
	}, forwardStatus(progrock.RecorderFromContext(ctx)))
	if err != nil {
		return fmt.Errorf("build llb def: %w", err)
	}

	_, err = runtime.Client.Solve(ctx, def, kitdclient.SolveOpt{
		LocalDirs:           localDirs,
		AllowedEntitlements: allowed,
		Session: []session.Attachable{
			authprovider.NewDockerAuthProvider(os.Stderr),
			secretsprovider.FromMap(secrets),
		},
		Exports: exports,
	}, forwardStatus(progrock.RecorderFromContext(ctx)))
	if err != nil {
		return err
	}

	return err
}

func (runtime *Buildkit) cacheResolver(inner llb.ImageMetaResolver) llb.ImageMetaResolver {
	return &cacheResolver{
		dbPath: filepath.Join(runtime.Config.Data, "refs.db"),
		inner:  inner,
	}
}

type builder struct {
	runtime  *Buildkit
	resolver llb.ImageMetaResolver

	secrets   map[string][]byte
	localDirs map[string]string
}

func (runtime *Buildkit) newBuilder(resolver llb.ImageMetaResolver) *builder {
	return &builder{
		runtime:  runtime,
		resolver: resolver,

		secrets:   map[string][]byte{},
		localDirs: map[string]string{},
	}
}

func (b *builder) llb(ctx context.Context, thunk bass.Thunk, captureStdout bool) (llb.ExecState, bool, error) {
	cmd, err := NewCommand(thunk)
	if err != nil {
		return llb.ExecState{}, false, err
	}

	imageRef, runState, needsInsecure, err := b.imageRef(ctx, thunk.Image)
	if err != nil {
		return llb.ExecState{}, false, err
	}

	id, err := thunk.SHA256()
	if err != nil {
		return llb.ExecState{}, false, err
	}

	runOpt := []llb.RunOption{
		llb.Hostname(id),
		llb.AddMount("/tmp", llb.Scratch(), llb.Tmpfs()),
		llb.AddMount("/dev/shm", llb.Scratch(), llb.Tmpfs()),
		llb.AddMount(workDir, runState),
		llb.AddMount(ioDir, llb.Scratch()),
		llb.AddMount(runExe, b.shim(), llb.SourcePath("run")),
	}

	if b.runtime.Config.DisableCache {
		runOpt = append(runOpt, llb.IgnoreCache)
	}

	if thunk.Insecure {
		needsInsecure = true

		runOpt = append(runOpt,
			llb.WithCgroupParent(id),
			llb.Security(llb.SecurityModeInsecure))
	}

	for _, mount := range cmd.Mounts {
		mountOpt, ni, err := b.initializeMount(ctx, workDir, mount)
		if err != nil {
			return llb.ExecState{}, false, err
		}

		if ni {
			needsInsecure = true
		}

		runOpt = append(runOpt, mountOpt)
	}

	var cwd string
	if cmd.Dir != nil {
		if filepath.IsAbs(*cmd.Dir) {
			cwd = *cmd.Dir
		} else {
			cwd = filepath.Join(workDir, *cmd.Dir)
		}
	} else {
		cwd = workDir
	}

	runOpt = append(runOpt, llb.With(llb.Dir(cwd)))

	if len(cmd.Stdin) > 0 {
		stdinBuf := new(bytes.Buffer)
		enc := json.NewEncoder(stdinBuf)
		for _, val := range cmd.Stdin {
			err := enc.Encode(val)
			if err != nil {
				return llb.ExecState{}, false, fmt.Errorf("encode stdin: %w", err)
			}
		}

		b.secrets[id] = stdinBuf.Bytes()

		runOpt = append(runOpt, llb.AddSecret(inputFile, llb.SecretID(id)))
		runOpt = append(runOpt, llb.AddEnv("_BASS_INPUT", inputFile))
	}

	if captureStdout {
		runOpt = append(runOpt, llb.AddEnv("_BASS_OUTPUT", outputFile))
	}

	for _, env := range cmd.Env {
		segs := strings.SplitN(env, "=", 2)
		runOpt = append(runOpt, llb.AddEnv(segs[0], segs[1]))
	}

	runOpt = append(
		runOpt,
		llb.Args(append([]string{runExe}, cmd.Args...)),
		llb.WithCustomName(strings.Join(cmd.Args, " ")),
	)

	return imageRef.Run(runOpt...), needsInsecure, nil
}

func (b *builder) shim() llb.State {
	shimBuilderImage := "golang:alpine"

	return llb.Image(
		shimBuilderImage,
		llb.WithMetaResolver(b.resolver),
		llb.Platform(b.runtime.Platform),
		llb.ResolveDigest(true),
		llb.WithCustomName("[hide] fetch bass shim builder image"),
	).
		File(llb.Mkfile("main.go", 0644, shim), llb.WithCustomName("[hide] load bass shim source")).
		Run(
			llb.AddMount("/bass", llb.Scratch()),
			llb.AddEnv("CGO_ENABLED", "0"),
			llb.Args([]string{"go", "build", "-o", "/bass/run", "main.go"}),
			llb.WithCustomName("[hide] compile bass shim"),
		).
		GetMount("/bass")
}

func (b *builder) imageRef(ctx context.Context, image *bass.ThunkImage) (llb.State, llb.State, bool, error) {
	if image == nil {
		// TODO: test
		return llb.Scratch(), llb.Scratch(), false, nil
	}

	if image.Ref != nil {
		return llb.Image(
			image.Ref.Ref(),
			llb.WithMetaResolver(b.resolver),
			llb.Platform(b.runtime.Platform),
		), llb.Scratch(), false, nil
	}

	if image.Thunk == nil {
		return llb.State{}, llb.State{}, false, fmt.Errorf("unsupported image type: %+v", image)
	}

	execState, needsInsecure, err := b.llb(ctx, *image.Thunk, false)
	if err != nil {
		return llb.State{}, llb.State{}, false, fmt.Errorf("image thunk llb: %w", err)
	}

	return execState.State, execState.GetMount(workDir), needsInsecure, nil
}

func (b *builder) initializeMount(ctx context.Context, runDir string, mount CommandMount) (llb.RunOption, bool, error) {
	var targetPath string
	if filepath.IsAbs(mount.Target) {
		targetPath = mount.Target
	} else {
		targetPath = filepath.Join(runDir, mount.Target)
	}

	if mount.Source.ThunkPath != nil {
		thunkSt, needsInsecure, err := b.llb(ctx, mount.Source.ThunkPath.Thunk, false)
		if err != nil {
			return nil, false, fmt.Errorf("thunk llb: %w", err)
		}

		return llb.AddMount(
			targetPath,
			thunkSt.GetMount(runDir),
			llb.SourcePath(mount.Source.ThunkPath.Path.FilesystemPath().FromSlash()),
		), needsInsecure, nil
	}

	if mount.Source.HostPath != nil {
		contextDir := mount.Source.HostPath.ContextDir
		b.localDirs[contextDir] = mount.Source.HostPath.ContextDir

		var excludes []string
		ignorePath := filepath.Join(contextDir, ".bassignore")
		ignore, err := os.Open(ignorePath)
		if err == nil {
			excludes, err = dockerignore.ReadAll(ignore)
			if err != nil {
				return nil, false, fmt.Errorf("parse %s: %w", ignorePath, err)
			}
		}

		sourcePath := mount.Source.HostPath.Path.FilesystemPath().FromSlash()

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
		), false, nil
	}

	if mount.Source.Cache != nil {
		return llb.AddMount(
			targetPath,
			llb.Scratch(),
			llb.AsPersistentCacheDir(mount.Source.Cache.String(), llb.CacheMountShared),
		), false, nil
	}

	return nil, false, fmt.Errorf("unrecognized mount source: %v", mount.Source)
}

func hash(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

type cacheResolver struct {
	dbPath string
	inner  llb.ImageMetaResolver
}

func (resolver *cacheResolver) ResolveImageConfig(ctx context.Context, ref string, opt llb.ResolveImageConfigOpt) (digest.Digest, []byte, error) {
	db, err := bolt.Open(resolver.dbPath, 0600, nil)
	if err != nil {
		return "", nil, fmt.Errorf("open db %s: %w", resolver.dbPath, err)
	}

	defer db.Close()

	var dig digest.Digest
	var config []byte
	var found bool
	_ = db.View(func(tx *bolt.Tx) error {
		digests := tx.Bucket([]byte(digestBucket))
		if digests == nil {
			return nil
		}

		digestCache := digests.Get([]byte(ref))
		if digestCache == nil {
			return nil
		}

		dig = digest.Digest(string(digestCache))

		configs := tx.Bucket([]byte(configBucket))
		if configs == nil {
			return nil
		}

		configCache := configs.Get(digestCache)
		if configCache == nil {
			return nil
		}

		config = make([]byte, len(configCache))
		copy(config, configCache)

		found = true
		return nil
	})

	if found {
		return dig, config, nil
	}

	dig, config, err = resolver.inner.ResolveImageConfig(ctx, ref, opt)
	if err != nil {
		return "", nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		digests, err := tx.CreateBucketIfNotExists([]byte(digestBucket))
		if err != nil {
			return err
		}

		configs, err := tx.CreateBucketIfNotExists([]byte(configBucket))
		if err != nil {
			return err
		}

		if err := digests.Put([]byte(ref), []byte(dig)); err != nil {
			return err
		}

		if err := configs.Put([]byte(dig), config); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return "", nil, fmt.Errorf("store ref cache: %w", err)
	}

	return dig, config, nil
}

func forwardStatus(rec *progrock.Recorder) chan *kitdclient.SolveStatus {
	statuses := make(chan *kitdclient.SolveStatus)

	go func() {
		for {
			s, ok := <-statuses
			if !ok {
				break
			}

			vs := make([]*graph.Vertex, len(s.Vertexes))
			for i, v := range s.Vertexes {
				vs[i] = (*graph.Vertex)(v)
			}

			ss := make([]*graph.VertexStatus, len(s.Statuses))
			for i, s := range s.Statuses {
				ss[i] = (*graph.VertexStatus)(s)
			}

			ls := make([]*graph.VertexLog, len(s.Logs))
			for i, l := range s.Logs {
				ls[i] = (*graph.VertexLog)(l)
			}

			rec.Record(&graph.SolveStatus{
				Vertexes: vs,
				Statuses: ss,
				Logs:     ls,
			})
		}
	}()

	return statuses
}
