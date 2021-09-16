package runtimes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/adrg/xdg"
	"github.com/concourse/go-archive/tarfs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dmount "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gofrs/flock"
	"github.com/mitchellh/go-homedir"
	"github.com/opencontainers/go-digest"
	"github.com/vito/bass"
	"github.com/vito/bass/zapctx"
	"github.com/vito/progrock"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Docker struct {
	External bass.Runtime
	Client   *client.Client
	Config   DockerConfig
}

var _ bass.Runtime = &Docker{}

const DockerName = "docker"

func init() {
	bass.RegisterRuntime(DockerName, NewDocker)
}

func NewDocker(external bass.Runtime, cfg *bass.Scope) (bass.Runtime, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}

	var config DockerConfig
	if cfg != nil {
		err = cfg.Decode(&config)
		if err != nil {
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

	for _, dir := range []string{artifactsDir, locksDir, responsesDir, logsDir} {
		err := os.MkdirAll(filepath.Join(config.Data, dir), 0700)
		if err != nil {
			return nil, err
		}
	}

	return &Docker{
		External: external,
		Client:   cli,
		Config:   config,
	}, nil
}

func (runtime *Docker) Run(ctx context.Context, w io.Writer, workload bass.Workload) (err error) {
	rec, err := workload.Vertex(progrock.RecorderFromContext(ctx))
	if err != nil {
		return fmt.Errorf("init workload recorder: %w", err)
	}

	defer func() {
		rec.Done(err)
	}()

	name, err := workload.SHA1()
	if err != nil {
		return fmt.Errorf("name: %w", err)
	}

	responsePath, err := runtime.Config.ResponsePath(name)
	if err != nil {
		return err
	}

	resFile, err := os.Open(responsePath)
	if err == nil {
		defer resFile.Close()

		rec.Cached()

		_, err = io.Copy(w, resFile)
		if err != nil {
			return err
		}

		logPath, err := runtime.Config.LogPath(name)
		if err != nil {
			return err
		}

		logFile, err := os.Open(logPath)
		if err != nil {
			return err
		}

		defer logFile.Close()

		_, err = io.Copy(rec.Stderr(), logFile)
		if err != nil {
			return err
		}

		return nil
	}

	err = runtime.run(ctx, w, workload, rec)
	if err != nil {
		rec.Error(err)
		return err
	}

	return nil
}

func (runtime *Docker) Load(ctx context.Context, workload bass.Workload) (*bass.Scope, error) {
	// TODO: run workload, parse response stream as bindings mapped to paths for
	// constructing workloads inheriting from the initial workload
	return nil, nil
}

func (runtime *Docker) Export(ctx context.Context, w io.Writer, workload bass.Workload, path bass.FilesystemPath) error {
	name, err := workload.SHA1()
	if err != nil {
		return fmt.Errorf("name: %w", err)
	}

	artifacts, err := runtime.Config.ArtifactsPath(name, path)
	if err != nil {
		return err
	}

	if _, err := os.Stat(artifacts); err != nil {
		err := runtime.Run(ctx, ioutil.Discard, workload)
		if err != nil {
			return fmt.Errorf("run export workload: %w", err)
		}
	}

	if path.IsDir() {
		return tarfs.Compress(w, artifacts, ".")
	} else {
		f, err := os.Open(artifacts)
		if err != nil {
			return err
		}

		_, err = io.Copy(w, f)
		if err != nil {
			return err
		}

		return nil
	}
}

func (runtime *Docker) run(ctx context.Context, w io.Writer, workload bass.Workload, rec *progrock.VertexRecorder) error {
	logger := zapctx.FromContext(ctx)

	name, err := workload.SHA1()
	if err != nil {
		return fmt.Errorf("name: %w", err)
	}

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

	dataDir, err := runtime.Config.ArtifactsPath(name, bass.DirPath{Path: "."})
	if err != nil {
		return err
	}

	err = os.MkdirAll(dataDir, 0700)
	if err != nil {
		return err
	}

	var runDir string
	if goruntime.GOOS == "windows" {
		runDir = `C:\tmp\run`
	} else {
		runDir = "/tmp/run"
	}

	imageName, err := runtime.imageRef(ctx, workload.Image, rec)
	if err != nil {
		return err
	}

	cmd, err := NewCommand(workload)
	if err != nil {
		return err
	}

	mounts := []dmount.Mount{
		{
			Type:   dmount.TypeBind,
			Source: dataDir,
			Target: runDir,
		},
		{
			Type:   dmount.TypeTmpfs,
			Target: "/dev/shm",
			TmpfsOptions: &dmount.TmpfsOptions{
				Mode: 01777,
			},
		},
	}

	for _, m := range cmd.Mounts {
		mount, err := runtime.initializeMount(ctx, dataDir, runDir, m, rec)
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

	var containerID string
	err = rec.Task("create container").Wrap(func() error {
		created, err := runtime.Client.ContainerCreate(ctx, cfg, hostCfg, nil, nil, name)
		if err != nil {
			return err
		}

		containerID = created.ID
		return nil
	})
	if err != nil {
		return fmt.Errorf("container create: %w", err)
	}

	defer func() {
		removeErr := runtime.Client.ContainerRemove(context.Background(), containerID, types.ContainerRemoveOptions{
			Force: true,
		})
		if removeErr != nil {
			rec.Error(fmt.Errorf("remove container: %w", err))
		}
	}()

	res, err := runtime.Client.ContainerAttach(ctx, containerID, types.ContainerAttachOptions{
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

	resC, errC := runtime.Client.ContainerWait(ctx, containerID, container.WaitConditionNextExit)

	run := rec.Task("run %s", strings.Join(cfg.Cmd, " "))

	run.Start()
	defer run.Complete()

	err = runtime.Client.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
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

	responseFile, err := os.Create(responsePath)
	if err != nil {
		return err
	}

	defer responseFile.Close()

	responseW := io.MultiWriter(responseFile, w)

	stdoutW := io.MultiWriter(logFile, rec.Stdout())
	stderrW := io.MultiWriter(logFile, rec.Stderr())
	if workload.Response.Stdout {
		stdoutW = responseW
	}

	eg := new(errgroup.Group)
	eg.Go(func() error {
		_, err := stdcopy.StdCopy(stdoutW, stderrW, res.Reader)
		return err
	})

	defer func() {
		err := eg.Wait()
		if err != nil {
			logger.Error("stream error", zap.Error(err))
		}
	}()

	select {
	case res := <-resC:
		if res.Error != nil {
			return fmt.Errorf("wait: %w", err)
		}

		if workload.Response.ExitCode {
			err = json.NewEncoder(responseW).Encode(res.StatusCode)
			if err != nil {
				return err
			}
		} else if res.StatusCode != 0 {
			return fmt.Errorf("exit status %d", res.StatusCode)
		}

	case err := <-errC:
		return fmt.Errorf("run: %w", err)

	case <-ctx.Done():
		err := runtime.Client.ContainerKill(context.Background(), containerID, "")
		if err != nil {
			return fmt.Errorf("stop container: %w", err)
		}

		return ctx.Err()
	}

	if workload.Response.File != nil {
		responseSrc, err := os.Open(filepath.Join(dataDir, workload.Response.File.FromSlash()))
		if err != nil {
			return err
		}

		_, err = io.Copy(responseW, responseSrc)
		if err != nil {
			return err
		}
	}

	return nil
}

func (runtime *Docker) initializeMount(ctx context.Context, dataDir, runDir string, mount CommandMount, rec *progrock.VertexRecorder) (_ dmount.Mount, err error) {
	task := rec.Task("mount %s to %s", mount.Source.ToValue(), mount.Target)

	task.Start()
	defer func() { task.Done(err) }()

	if mount.Source.LocalPath != nil {
		fsp := mount.Source.LocalPath.FilesystemPath()
		hostPath := filepath.Join(dataDir, fsp.FromSlash())
		if fsp.IsDir() {
			err := os.MkdirAll(hostPath, 0700)
			if err != nil {
				return dmount.Mount{}, fmt.Errorf("create mount source dir: %w", err)
			}
		} else {
			err := os.WriteFile(hostPath, nil, 0600)
			if err != nil {
				return dmount.Mount{}, fmt.Errorf("create mount source file: %w", err)
			}
		}

		return dmount.Mount{
			Type:   dmount.TypeBind,
			Source: hostPath,
			Target: filepath.Join(runDir, mount.Target),
		}, nil
	}

	if mount.Source.WorkloadPath == nil {
		return dmount.Mount{}, fmt.Errorf("unknown mount source type: %+v", mount.Source)
	}

	artifact := mount.Source.WorkloadPath

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
		err := runtime.External.Run(ctx, ioutil.Discard, artifact.Workload)
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

func (runtime *Docker) imageRef(ctx context.Context, image *bass.ImageEnum, rec *progrock.VertexRecorder) (string, error) {
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

		task := rec.Task("pull " + renderImage(image))

		task.Start()
		defer task.Complete()

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

			err = handleMessage(msg, rec)
			if err != nil {
				return "", fmt.Errorf("pull: %w", err)
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
		return imageName, nil
	}

	if !client.IsErrNotFound(err) {
		return "", fmt.Errorf("check if image exists: %w", err)
	}

	r, w := io.Pipe()
	go func() {
		w.CloseWithError(
			runtime.External.Export(
				ctx,
				w,
				image.Path.Workload,
				image.Path.Path.FilesystemPath(),
			),
		)
	}()

	task := rec.Task("load " + renderImage(image))

	task.Start()
	defer task.Complete()

	resp, err := runtime.Client.ImageLoad(ctx, r, false)
	if err != nil {
		return "", fmt.Errorf("load image: %w", err)
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

		err = handleMessage(msg, rec)
		if err != nil {
			return "", fmt.Errorf("load: %w", err)
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

func handleMessage(msg jsonmessage.JSONMessage, rec *progrock.VertexRecorder) error {
	if msg.ID == "" {
		return nil
	}

	task := rec.Task("layer %s", msg.ID)

	if msg.Error != nil {
		return fmt.Errorf("pull: %w", msg.Error)
	}

	if msg.Progress != nil {
		if task.Status.Started == nil {
			task.Start()
		}

		if msg.Progress.Total != 0 {
			task.Progress(msg.Progress.Current, msg.Progress.Total)

			if msg.Progress.Current == msg.Progress.Total {
				task.Complete()
			}
		}
	}

	return nil
}

type DockerConfig struct {
	Data string `json:"data,omitempty"`
}

const artifactsDir = "artifacts"
const locksDir = "locks"
const responsesDir = "responses"
const logsDir = "logs"

func (config DockerConfig) ArtifactsPath(id string, path bass.FilesystemPath) (string, error) {
	return config.path(artifactsDir, id, path.FromSlash())
}

func (config DockerConfig) LockPath(id string) (string, error) {
	return config.path(locksDir, id+".lock")
}

func (config DockerConfig) ResponsePath(id string) (string, error) {
	return config.path(responsesDir, id)
}

func (config DockerConfig) LogPath(id string) (string, error) {
	return config.path(logsDir, id)
}

func (config DockerConfig) path(path ...string) (string, error) {
	return filepath.Abs(
		filepath.Join(
			append(
				[]string{config.Data},
				path...,
			)...,
		),
	)
}

func renderImage(image *bass.ImageEnum) string {
	if image == nil {
		return "(none)"
	}

	if image.Ref != nil {
		if image.Ref.Tag != "" {
			return image.Ref.Repository + ":" + image.Ref.Tag
		} else {
			return image.Ref.Repository + ":latest"
		}
	} else if image.Path != nil {
		sum, err := image.Path.Workload.SHA256()
		if err != nil {
			return image.ToValue().String()
		}

		dig := digest.NewDigestFromEncoded(digest.SHA256, sum)

		return fmt.Sprintf("%s/%s", dig, image.Path.Path.ToValue())
	} else {
		return image.ToValue().String()
	}
}
