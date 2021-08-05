package docker

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/concourse/go-archive/tarfs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dmount "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gofrs/flock"
	"github.com/mattn/go-isatty"
	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/bass/zapctx"
	"go.uber.org/zap"
)

type Runtime struct {
	Pool   *runtimes.Pool
	Client *client.Client
	Config RuntimeConfig
}

var _ runtimes.Runtime = &Runtime{}

func init() {
	runtimes.Register("docker", NewRuntime)
}

func NewRuntime(pool *runtimes.Pool, cfg bass.Object) (runtimes.Runtime, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}

	var config RuntimeConfig
	err = cfg.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("docker runtime config: %w", err)
	}

	return &Runtime{
		Pool:   pool,
		Client: cli,
		Config: config,
	}, nil
}

func (runtime *Runtime) Run(ctx context.Context, workload bass.Workload) error {
	logger := zapctx.FromContext(ctx)

	name, err := workload.SHA1()
	if err != nil {
		return fmt.Errorf("name: %w", err)
	}

	logger = logger.With(zap.String("workload", name))

	lockPath, err := runtime.Config.LockPath(name)
	if err != nil {
		return err
	}

	lock := flock.New(lockPath)

	err = lock.Lock()
	if err != nil {
		return err
	}

	defer lock.Unlock()

	responsePath, err := runtime.Config.ResponsePath(name)
	if err != nil {
		return err
	}

	if _, err := os.Stat(responsePath); err == nil {
		logger.Debug("cached", zap.String("response", responsePath))

		logPath, err := runtime.Config.LogPath(name)
		if err != nil {
			return err
		}

		logFile, err := os.Open(logPath)
		if err != nil {
			return err
		}

		defer logFile.Close()

		_, err = io.Copy(os.Stderr, logFile)
		if err != nil {
			return err
		}

		return nil
	}

	logger.Info("running")

	return runtime.run(zapctx.ToContext(ctx, logger), name, workload)
}

func (runtime *Runtime) Response(ctx context.Context, w io.Writer, workload bass.Workload) error {
	name, err := workload.SHA1()
	if err != nil {
		return fmt.Errorf("name: %w", err)
	}

	responsePath, err := runtime.Config.ResponsePath(name)
	if err != nil {
		return err
	}

	resFile, err := os.Open(responsePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	defer resFile.Close()

	_, err = io.Copy(w, resFile)
	return err
}

func (runtime *Runtime) Export(ctx context.Context, w io.Writer, workload bass.Workload, path bass.FilesystemPath) error {
	name, err := workload.SHA1()
	if err != nil {
		return fmt.Errorf("name: %w", err)
	}

	artifact, err := runtime.Config.ArtifactsPath(name, path)
	if err != nil {
		return err
	}

	var workDir, files string
	if path.IsDir() {
		workDir = artifact
		files = "."
	} else {
		workDir = filepath.Dir(artifact)
		files = filepath.Base(artifact)
	}

	return tarfs.Compress(w, workDir, files)
}

func (runtime *Runtime) run(ctx context.Context, name string, workload bass.Workload) error {
	logger := zapctx.FromContext(ctx)

	dataDir, err := runtime.Config.ArtifactsPath(name, bass.DirPath{Path: "."})
	if err != nil {
		return err
	}

	err = os.MkdirAll(dataDir, 0755)
	if err != nil {
		return err
	}

	var runDir string
	if goruntime.GOOS == "windows" {
		runDir = `C:\tmp\run`
	} else {
		runDir = "/tmp/run"
	}

	imageName, err := runtime.imageRef(ctx, workload.Image)
	if err != nil {
		return err
	}

	cmd, err := workload.Resolve()
	if err != nil {
		return err
	}

	mounts := []dmount.Mount{
		{
			Type:   dmount.TypeBind,
			Source: dataDir,
			Target: runDir,
		},
	}

	for _, m := range cmd.Mounts {
		mount, err := runtime.initializeMount(ctx, runDir, m)
		if err != nil {
			return fmt.Errorf("mount %s: %w", m.Target, err)
		}

		mounts = append(mounts, mount)
	}

	var cwd string
	if cmd.Dir != nil {
		if filepath.IsAbs(*cmd.Dir) {
			cwd = *cmd.Dir
		} else {
			cwd = filepath.Join(runDir, *cmd.Dir)
		}
	} else {
		cwd = runDir
	}

	cfg := &container.Config{
		Image:        imageName,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    true,
		Entrypoint:   cmd.Entrypoint,
		Cmd:          cmd.Args,
		Env:          cmd.Env,
		WorkingDir:   cwd,
		Labels: map[string]string{
			"bass": "yes",
		},
	}

	hostCfg := &container.HostConfig{
		Mounts:     mounts,
		Privileged: workload.Insecure,
	}

	created, err := runtime.Client.ContainerCreate(ctx, cfg, hostCfg, nil, nil, name)
	if err != nil {
		return fmt.Errorf("container create: %w", err)
	}

	logger = logger.With(zap.String("container", created.ID))

	defer func() {
		err := runtime.Client.ContainerRemove(context.Background(), created.ID, types.ContainerRemoveOptions{
			Force: true,
		})
		if err != nil {
			logger.Error("failed to remove container", zap.Error(err))
		}

		logger.Debug("removed")
	}()

	logger.Info("created")

	res, err := runtime.Client.ContainerAttach(ctx, created.ID, types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		Logs:   true,
	})
	if err != nil {
		return fmt.Errorf("container attach: %w", err)
	}

	defer res.Close()

	resC, errC := runtime.Client.ContainerWait(ctx, created.ID, container.WaitConditionNextExit)

	err = runtime.Client.ContainerStart(ctx, created.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("container start: %w", err)
	}

	enc := json.NewEncoder(res.Conn)

	for _, val := range cmd.Stdin {
		err := enc.Encode(val)
		if err != nil {
			return fmt.Errorf("write request: %w", err)
		}
	}

	err = res.CloseWrite()
	if err != nil {
		return fmt.Errorf("close write: %w", err)
	}

	logPath, err := runtime.Config.LogPath(name)
	if err != nil {
		return err
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}

	defer logFile.Close()

	responsePath, err := runtime.Config.ResponsePath(name)
	if err != nil {
		return err
	}

	outStream := io.MultiWriter(logFile, os.Stdout)
	if workload.Response.Stdout {
		responseFile, err := os.Create(responsePath)
		if err != nil {
			return err
		}

		defer responseFile.Close()

		outStream = responseFile
	}

	_, err = stdcopy.StdCopy(
		outStream,
		io.MultiWriter(logFile, os.Stderr),
		res.Reader,
	)
	if err != nil {
		return fmt.Errorf("stream output: %w", err)
	}

	select {
	case res := <-resC:
		if res.Error != nil {
			return fmt.Errorf("wait: %w", err)
		}

		if workload.Response.ExitCode {
			responseFile, err := os.Create(responsePath)
			if err != nil {
				return err
			}

			err = json.NewEncoder(responseFile).Encode(res.StatusCode)
			if err != nil {
				return err
			}

			err = responseFile.Close()
			if err != nil {
				return err
			}
		} else if res.StatusCode != 0 {
			return fmt.Errorf("exit status %d", res.StatusCode)
		}

	case err := <-errC:
		return fmt.Errorf("run: %w", err)
	}

	if workload.Response.File != nil {
		err = os.Symlink(
			filepath.Join(dataDir, workload.Response.File.FromSlash()),
			responsePath,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (runtime *Runtime) initializeMount(ctx context.Context, runDir string, mount bass.CommandMount) (dmount.Mount, error) {
	artifact := mount.Source
	subPath := artifact.Path.FilesystemPath()

	name, err := artifact.Workload.SHA1()
	if err != nil {
		return dmount.Mount{}, err
	}

	hostPath, err := runtime.Config.ArtifactsPath(name, subPath)
	if err != nil {
		return dmount.Mount{}, err
	}

	if _, err := os.Stat(hostPath); err != nil {
		err := runtime.Pool.Run(ctx, artifact.Workload)
		if err != nil {
			return dmount.Mount{}, fmt.Errorf("run input workload: %w", err)
		}

		// TODO: stat hostPath again; if still not found, export it
	}

	return dmount.Mount{
		Type:   dmount.TypeBind,
		Source: hostPath,
		Target: filepath.Join(runDir, mount.Target),
	}, nil
}

func (runtime *Runtime) imageRef(ctx context.Context, image *bass.ImageEnum) (string, error) {
	logger := zapctx.FromContext(ctx)

	if image == nil {
		return "", fmt.Errorf("no image provided")
	}

	if image.Ref != nil {
		imageName := image.Ref.Repository

		_, _, err := runtime.Client.ImageInspectWithRaw(ctx, imageName)
		if err == nil {
			// already pulled
			return imageName, nil
		}

		rc, err := runtime.Client.ImagePull(ctx, imageName, types.ImagePullOptions{})
		if err != nil {
			return "", fmt.Errorf("pull image: %w", err)
		}

		dec := json.NewDecoder(rc)

		for {
			var msg jsonmessage.JSONMessage
			err := dec.Decode(&msg)
			if err != nil {
				if err == io.EOF {
					break
				}

				return "", fmt.Errorf("decode docker response: %w", err)
			}

			err = msg.Display(os.Stderr, isatty.IsTerminal(os.Stderr.Fd()))
			if err != nil {
				return "", fmt.Errorf("error response: %w", err)
			}
		}

		return imageName, nil
	}

	if image.Path == nil {
		return "", fmt.Errorf("unsupported image type: %+v", image)
	}

	imageWorkloadName, err := image.Path.Workload.SHA1()
	if err != nil {
		return "", err
	}

	imageName := imageWorkloadName

	_, _, err = runtime.Client.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		logger.Debug("using imported image", zap.String("image", imageName))
		return imageName, nil
	}

	if !client.IsErrNotFound(err) {
		return "", fmt.Errorf("check if image exists: %w", err)
	}

	logger.Info("importing image", zap.String("image", imageName))

	r, w := io.Pipe()
	go func() {
		w.CloseWithError(
			runtime.Pool.Export(
				ctx,
				w,
				image.Path.Workload,
				image.Path.Path.FilesystemPath(),
			),
		)
	}()

	tr := tar.NewReader(r)

	_, err = tr.Next()
	if err != nil {
		return "", fmt.Errorf("export oci archive: %w", err)
	}

	resp, err := runtime.Client.ImageLoad(ctx, tr, false)
	if err != nil {
		return "", fmt.Errorf("import image: %w", err)
	}

	if !resp.JSON {
		return "", fmt.Errorf("bad response (no JSON)")
	}

	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)

	var imageRef string
	for {
		var msg jsonmessage.JSONMessage
		err := dec.Decode(&msg)
		if err != nil {
			if err == io.EOF {
				return "", fmt.Errorf("could not determine image name from docker response stream")
			}

			return "", fmt.Errorf("decode docker response: %w", err)
		}

		err = msg.Display(os.Stderr, isatty.IsTerminal(os.Stderr.Fd()))
		if err != nil {
			return "", fmt.Errorf("error response: %w", err)
		}

		if strings.HasPrefix(msg.Stream, "Loaded") {
			segs := strings.Fields(msg.Stream)
			imageRef = segs[len(segs)-1]
			break
		}
	}

	err = runtime.Client.ImageTag(ctx, imageRef, imageName)
	if err != nil {
		return "", fmt.Errorf("tag image %q as %q: %w", imageRef, imageName, err)
	}

	return imageName, nil
}
