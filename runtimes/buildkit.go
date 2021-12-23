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

	"github.com/adrg/xdg"
	"github.com/mitchellh/go-homedir"
	kitdclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/opencontainers/go-digest"
	"github.com/vito/bass"
	"github.com/vito/progrock"
	"github.com/vito/progrock/graph"
	bolt "go.etcd.io/bbolt"

	_ "embed"
)

type Buildkit struct {
	Config BuildkitConfig

	resolver llb.ImageMetaResolver
}

type BuildkitConfig struct {
	BuildkitAddr string `json:"buildkit_addr,omitempty"`
	Data         string `json:"data,omitempty"`
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
	bass.RegisterRuntime(BuildkitName, NewBuildkit)
}

func NewBuildkit(_ bass.Runtime, cfg *bass.Scope) (bass.Runtime, error) {
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

	dbPath := filepath.Join(dataRoot, "refs.db")

	return &Buildkit{
		Config: config,

		resolver: &cacheResolver{
			dbPath: dbPath,
			inner:  imagemetaresolver.Default(),
		},
	}, nil
}

func (runtime *Buildkit) dialBuildkit() (*kitdclient.Client, error) {
	addr := runtime.Config.BuildkitAddr
	if addr == "" {
		sockPath, err := xdg.SearchRuntimeFile("buildkit/buildkitd.sock")
		if err == nil {
			// support respecting XDG_RUNTIME_DIR instead of assuming /run/
			addr = "unix://" + sockPath
		}
	}

	return kitdclient.New(context.TODO(), addr)
}

func (runtime *Buildkit) Run(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	id, err := thunk.SHA256()
	if err != nil {
		return err
	}

	err = runtime.build(
		ctx,
		thunk,
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

func (runtime *Buildkit) build(ctx context.Context, thunk bass.Thunk, transform func(llb.ExecState) marshalable, exports ...kitdclient.ExportEntry) error {
	b := newBuilder(runtime.resolver)
	st, err := b.llb(ctx, thunk)
	if err != nil {
		return err
	}

	def, err := transform(st).Marshal(ctx)
	if err != nil {
		return err
	}

	client, err := runtime.dialBuildkit()
	if err != nil {
		return fmt.Errorf("dial buildkit: %w", err)
	}

	defer client.Close()

	_, err = client.Solve(ctx, def, kitdclient.SolveOpt{
		LocalDirs: b.localDirs,
		AllowedEntitlements: []entitlements.Entitlement{
			entitlements.EntitlementSecurityInsecure,
		},
		Session: []session.Attachable{
			authprovider.NewDockerAuthProvider(os.Stderr),
			secretsprovider.FromMap(b.secrets),
		},
		Exports: exports,
	}, forwardStatus(progrock.RecorderFromContext(ctx)))
	if err != nil {
		return err
	}

	return err
}

type builder struct {
	secrets   map[string][]byte
	localDirs map[string]string
	resolver  llb.ImageMetaResolver
}

func newBuilder(resolver llb.ImageMetaResolver) *builder {
	return &builder{
		secrets:   map[string][]byte{},
		localDirs: map[string]string{},
		resolver:  resolver,
	}
}

func (b *builder) llb(ctx context.Context, thunk bass.Thunk) (llb.ExecState, error) {
	cmd, err := NewCommand(thunk)
	if err != nil {
		return llb.ExecState{}, err
	}

	imageRef, runState, err := b.imageRef(ctx, thunk.Image)
	if err != nil {
		return llb.ExecState{}, err
	}

	id, err := thunk.SHA256()
	if err != nil {
		return llb.ExecState{}, err
	}

	runOpt := []llb.RunOption{
		llb.Hostname(id),
		llb.AddMount("/tmp", llb.Scratch(), llb.Tmpfs()),
		llb.AddMount("/dev/shm", llb.Scratch(), llb.Tmpfs()),
		llb.AddMount(workDir, runState),
		llb.AddMount(ioDir, llb.Scratch()),
		llb.AddMount(runExe, b.shim(), llb.SourcePath("run")),

		llb.AddEnv("_BASS_OUTPUT", outputFile),
	}

	if thunk.Insecure {
		runOpt = append(runOpt,
			llb.WithCgroupParent(id),
			llb.Security(llb.SecurityModeInsecure))
	}

	for _, mount := range cmd.Mounts {
		mountOpt, err := b.initializeMount(ctx, workDir, mount)
		if err != nil {
			return llb.ExecState{}, err
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
				return llb.ExecState{}, fmt.Errorf("encode stdin: %w", err)
			}
		}

		b.secrets[id] = stdinBuf.Bytes()

		runOpt = append(runOpt, llb.AddSecret(inputFile, llb.SecretID(id)))
		runOpt = append(runOpt, llb.AddEnv("_BASS_INPUT", inputFile))
	}

	if thunk.Response.Stdout {
		runOpt = append(runOpt, llb.AddEnv("_BASS_RESPONSE_SOURCE", "stdout"))
	} else if thunk.Response.ExitCode {
		runOpt = append(runOpt, llb.AddEnv("_BASS_RESPONSE_SOURCE", "exit"))
	} else if thunk.Response.File != nil {
		runOpt = append(runOpt, llb.AddEnv("_BASS_RESPONSE_SOURCE", "file:"+thunk.Response.File.String()))
	}

	if thunk.Response.Protocol != "" {
		runOpt = append(runOpt, llb.AddEnv("_BASS_RESPONSE_PROTOCOL", thunk.Response.Protocol))
	}

	runOpt = append(
		runOpt,
		llb.Args(append([]string{runExe}, cmd.Args...)),
	)

	for _, env := range cmd.Env {
		segs := strings.SplitN(env, "=", 2)
		runOpt = append(runOpt, llb.AddEnv(segs[0], segs[1]))
	}

	return imageRef.Run(runOpt...), nil
}

func (b *builder) shim() llb.State {
	return llb.Image(
		"golang:latest",
		llb.WithMetaResolver(b.resolver),
	).
		File(llb.Mkfile("main.go", 0644, shim)).
		Run(
			llb.AddMount("/bass", llb.Scratch()),
			llb.AddEnv("CGO_ENABLED", "0"),
			llb.Args([]string{"go", "build", "-o", "/bass/run", "main.go"}),
		).
		GetMount("/bass")
}

func (b *builder) imageRef(ctx context.Context, image *bass.ImageEnum) (llb.State, llb.State, error) {
	if image == nil {
		// TODO: test
		return llb.Scratch(), llb.Scratch(), nil
	}

	if image.Ref != nil {
		tag := image.Ref.Tag
		if tag == "" {
			tag = "latest"
		}

		return llb.Image(
			fmt.Sprintf("%s:%s", image.Ref.Repository, tag),
			llb.WithMetaResolver(b.resolver),
		), llb.Scratch(), nil
	}

	if image.Thunk == nil {
		return llb.State{}, llb.State{}, fmt.Errorf("unsupported image type: %+v", image)
	}

	execState, err := b.llb(ctx, *image.Thunk)
	if err != nil {
		return llb.State{}, llb.State{}, fmt.Errorf("image thunk llb: %w", err)
	}

	return execState.State, execState.GetMount(workDir), nil
}

func (b *builder) initializeMount(ctx context.Context, runDir string, mount CommandMount) (llb.RunOption, error) {
	var targetPath string
	if filepath.IsAbs(mount.Target) {
		targetPath = mount.Target
	} else {
		targetPath = filepath.Join(runDir, mount.Target)
	}

	if mount.Source.ThunkPath != nil {
		thunkSt, err := b.llb(ctx, mount.Source.ThunkPath.Thunk)
		if err != nil {
			return nil, fmt.Errorf("thunk llb: %w", err)
		}

		return llb.AddMount(
			targetPath,
			thunkSt.GetMount(runDir),
			llb.SourcePath(mount.Source.ThunkPath.Path.FilesystemPath().FromSlash()),
		), nil
	}

	if mount.Source.HostPath != nil {
		localName := mount.Source.HostPath.ContextDir
		b.localDirs[localName] = mount.Source.HostPath.ContextDir

		return llb.AddMount(
			targetPath,
			llb.Local(localName),
			llb.SourcePath(mount.Source.HostPath.Path.FilesystemPath().FromSlash()),
		), nil
	}

	return nil, fmt.Errorf("unrecognized mount source: %v", mount.Source)
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
