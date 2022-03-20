package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/oci/cas"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
)

type Command struct {
	Args  []string `json:"args"`
	Stdin []byte   `json:"stdin"`
	Env   []string `json:"env"`
	Dir   *string  `json:"dir"`
}

var stdoutPath string

func init() {
	stdoutPath = os.Getenv("_BASS_OUTPUT")
	os.Unsetenv("_BASS_OUTPUT")
}

func main() {
	if len(os.Args) == 1 {
		fmt.Fprintf(os.Stderr, "usage: %s <unpack|get-config|run>\n", os.Args[0])
		os.Exit(1)
	}

	var err error
	switch filepath.Base(os.Args[1]) {
	case "unpack":
		err = unpack(os.Args[1:])
	case "get-config":
		err = getConfig(os.Args[1:])
	case "run":
		os.Exit(run(os.Args[1:]))
		return
	default:
		fmt.Fprintf(os.Stderr, "usage: %s <unpack|get-config|run>\n", os.Args[0])
		os.Exit(1)
		return
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(args []string) int {
	runtime.GOMAXPROCS(1)

	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s cmd.json", args[0])
		return 1
	}

	cmdPath := args[1]

	cmdPayload, err := os.ReadFile(cmdPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read cmd: %s\n", err)
		return 1
	}

	var cmd Command
	err = json.Unmarshal(cmdPayload, &cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unmarshal cmd: %s\n", err)
		return 1
	}

	err = os.Remove(cmdPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "burn after reading: %s\n", err)
		return 1
	}

	var stdout io.Writer = os.Stdout
	if stdoutPath != "" {
		response, err := os.Create(stdoutPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create output error: %s\n", err)
			return 1
		}

		defer response.Close()

		stdout = response
	}

	for _, e := range cmd.Env {
		segs := strings.SplitN(e, "=", 2)
		if len(segs) != 2 {
			fmt.Fprintf(os.Stderr, "warning: malformed env")
			continue
		}

		os.Setenv(segs[0], segs[1])
	}

	bin := cmd.Args[0]
	argv := cmd.Args[1:]
	execCmd := exec.Command(bin, argv...)
	if cmd.Dir != nil {
		execCmd.Dir = *cmd.Dir
	}
	execCmd.Stdin = bytes.NewBuffer(cmd.Stdin)
	execCmd.Stdout = stdout
	execCmd.Stderr = os.Stderr
	err = execCmd.Run()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			// propagate exit status
			return exit.ExitCode()
		} else {
			fmt.Fprintf(os.Stderr, "run error: %s\n", err)
			return 1
		}
	}

	return 0
}

func getConfig(args []string) error {
	ctx := context.Background()

	if len(args) != 4 {
		return fmt.Errorf("usage: %s image.tar tag dest/", args[0])
	}

	archiveSrc := args[1]
	fromName := args[2]
	configDst := args[3]

	layout, err := openTar(archiveSrc)
	if err != nil {
		return fmt.Errorf("create layout: %w", err)
	}

	defer layout.Close()

	ext := casext.NewEngine(layout)

	mspec, err := loadManifest(ctx, ext, fromName)
	if err != nil {
		return err
	}

	config, err := ext.FromDescriptor(ctx, mspec.Config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if config.Descriptor.MediaType != ispec.MediaTypeImageConfig {
		return fmt.Errorf("bad config media type: %s", config.Descriptor.MediaType)
	}

	ispec := config.Data.(ispec.Image)

	configPath := filepath.Join(configDst, "config.json")

	configFile, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("create config.json: %w", err)
	}

	defer configFile.Close()

	err = json.NewEncoder(configFile).Encode(ispec.Config)
	if err != nil {
		return fmt.Errorf("encode image config: %w", err)
	}

	return nil
}

func unpack(args []string) error {
	ctx := context.Background()

	if len(args) != 4 {
		return fmt.Errorf("usage: %s image.tar tag dest/", args[0])
	}

	archiveSrc := args[1]
	fromName := args[2]
	rootfsPath := args[3]

	layout, err := openTar(archiveSrc)
	if err != nil {
		return fmt.Errorf("create layout: %w", err)
	}

	defer layout.Close()

	ext := casext.NewEngine(layout)

	mspec, err := loadManifest(ctx, ext, fromName)
	if err != nil {
		return err
	}

	err = layer.UnpackRootfs(context.Background(), ext, rootfsPath, mspec, &layer.UnpackOptions{})
	if err != nil {
		return fmt.Errorf("unpack rootfs: %w", err)
	}

	return nil
}

func loadManifest(ctx context.Context, ext casext.Engine, name string) (ispec.Manifest, error) {
	descPaths, err := ext.ResolveReference(context.Background(), name)
	if err != nil {
		return ispec.Manifest{}, fmt.Errorf("resolve ref: %w", err)
	}

	if len(descPaths) == 0 {
		return ispec.Manifest{}, fmt.Errorf("tag not found: %s", name)
	}

	if len(descPaths) != 1 {
		return ispec.Manifest{}, fmt.Errorf("ambiguous tag?: %s (%d paths returned)", name, len(descPaths))
	}

	manifest, err := ext.FromDescriptor(ctx, descPaths[0].Descriptor())
	if err != nil {
		return ispec.Manifest{}, fmt.Errorf("load manifest: %w", err)
	}

	if manifest.Descriptor.MediaType != ispec.MediaTypeImageManifest {
		return ispec.Manifest{}, fmt.Errorf("bad manifest media type: %s", manifest.Descriptor.MediaType)
	}

	return manifest.Data.(ispec.Manifest), nil
}

func openTar(tarPath string) (cas.Engine, error) {
	archive, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}

	return &tarEngine{archive}, nil
}

// tarEngine implements a read-only cas.Engine backed by a .tar archive.
type tarEngine struct {
	archive *os.File
}

func (engine *tarEngine) PutBlob(ctx context.Context, reader io.Reader) (digest.Digest, int64, error) {
	return "", 0, fmt.Errorf("PutBlob: %w", cas.ErrNotImplemented)
}

func (engine *tarEngine) GetBlob(ctx context.Context, dig digest.Digest) (io.ReadCloser, error) {
	r, err := engine.open(path.Join("blobs", dig.Algorithm().String(), dig.Encoded()))
	if err != nil {
		return nil, err
	}

	return io.NopCloser(r), nil
}

func (engine *tarEngine) StatBlob(ctx context.Context, dig digest.Digest) (bool, error) {
	_, err := engine.open(path.Join("blobs", dig.Algorithm().String(), dig.Encoded()))
	if err != nil {
		if errors.Is(err, cas.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (engine *tarEngine) PutIndex(ctx context.Context, index ispec.Index) error {
	return fmt.Errorf("PutIndex: %w", cas.ErrNotImplemented)
}

func (engine *tarEngine) GetIndex(ctx context.Context) (ispec.Index, error) {
	var idx ispec.Index
	r, err := engine.open("index.json")
	if err != nil {
		return ispec.Index{}, err
	}

	err = json.NewDecoder(r).Decode(&idx)
	if err != nil {
		return ispec.Index{}, err
	}

	return idx, nil
}

func (engine *tarEngine) open(p string) (io.Reader, error) {
	_, err := engine.archive.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	tr := tar.NewReader(engine.archive)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("open %s: %w", p, cas.ErrNotExist)
			}

			return nil, err
		}

		if path.Clean(hdr.Name) == p {
			return tr, nil
		}
	}
}

func (engine *tarEngine) DeleteBlob(ctx context.Context, digest digest.Digest) (err error) {
	return fmt.Errorf("DeleteBlob: %w", cas.ErrNotImplemented)
}

func (engine *tarEngine) ListBlobs(ctx context.Context) ([]digest.Digest, error) {
	_, err := engine.archive.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	tr := tar.NewReader(engine.archive)

	var digs []digest.Digest
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("next: %w", err)
		}

		if strings.HasPrefix(path.Clean(hdr.Name), "blobs/") {
			dir, encoded := path.Split(hdr.Name)
			_, alg := path.Split(dir)
			digs = append(digs, digest.NewDigestFromEncoded(digest.Algorithm(alg), encoded))
		}
	}

	return digs, nil
}

func (engine *tarEngine) Clean(ctx context.Context) error { return nil }

func (engine *tarEngine) Close() error {
	return engine.archive.Close()
}
