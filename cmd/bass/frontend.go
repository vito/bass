package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	gwclient "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/apicaps"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstls"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/runtimes/util"
)

func frontend(ctx context.Context) error {
	err := grpcclient.RunFromEnvironment(ctx, frontendBuild)
	if err != nil {
		cli.WriteError(ctx, err)
	}

	return err
}

// mimic dockerfile.v1 frontend
const (
	buildArgPrefix      = "build-arg:"
	localNameContext    = "context"
	localNameDockerfile = "dockerfile"
	localNameBassTLS    = "bass-tls"
	keyFilename         = "filename"
)

type InputsFilesystem struct {
	ctx    context.Context
	gw     gwclient.Client
	caps   apicaps.CapSet
	inputs map[string]llb.State
}

var _ bass.Filesystem = &InputsFilesystem{}

func (fs *InputsFilesystem) FS(contextDir string) (fs.FS, error) {
	input, found := fs.inputs[contextDir]
	if !found {
		return nil, fmt.Errorf("unknown input: %s", contextDir)
	}

	return util.OpenRefFS(fs.ctx, fs.gw, input, llb.WithCaps(fs.caps))
}

func (fs *InputsFilesystem) Write(path string, r io.Reader) error {
	return nil
}

func frontendBuild(ctx context.Context, gw gwclient.Client) (*gwclient.Result, error) {
	caps := gw.BuildOpts().Caps
	opts := gw.BuildOpts().Opts

	scriptFn := gw.BuildOpts().Opts[keyFilename]
	if scriptFn == "" {
		scriptFn = "Dockerfile"
	}

	inputs, err := gw.Inputs(ctx)
	if err != nil {
		return nil, err
	}

	_, found := inputs[localNameContext]
	if !found {
		// running from 'docker build', which doesn't set inputs
		inputs[localNameContext] = llb.Local(localNameContext,
			llb.SessionID(gw.BuildOpts().SessionID),
			llb.WithCustomName("[internal] local bass workdir"),
		)
	}

	scriptInput, found := inputs[localNameDockerfile]
	if !found {
		// running from 'docker build', which doesn't set inputs
		scriptInput = llb.Local(localNameDockerfile,
			llb.SessionID(gw.BuildOpts().SessionID),
			llb.WithCustomName("[internal] local bass script"),
		)

		inputs[localNameDockerfile] = scriptInput
	}

	// Override real filesystem with one that knows how to read directly from the
	// given inputs, and discard writes.
	bass.FS = &InputsFilesystem{
		ctx:    ctx,
		gw:     gw,
		caps:   caps,
		inputs: inputs,
	}

	var certsDir string
	certsInput, found := inputs[localNameBassTLS]
	if found {
		certFS, err := util.OpenRefFS(ctx, gw, certsInput, llb.WithCaps(caps))
		if err != nil {
			return nil, err
		}

		certsDir = basstls.DefaultDir
		if err := os.MkdirAll(certsDir, 0700); err != nil {
			return nil, err
		}

		for _, name := range basstls.CAFiles {
			source, err := certFS.Open(name)
			if err != nil {
				return nil, err
			}

			fi, err := source.Stat()
			if err != nil {
				return nil, err
			}

			target, err := os.OpenFile(path.Join(certsDir, name), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode())
			if err != nil {
				return nil, err
			}

			if _, err := io.Copy(target, source); err != nil {
				return nil, err
			}

			if err := target.Close(); err != nil {
				return nil, err
			}

			if err := source.Close(); err != nil {
				return nil, err
			}
		}
	}

	scriptFs, err := util.OpenRefFS(ctx, gw, scriptInput, llb.WithCaps(caps))
	if err != nil {
		return nil, err
	}

	// contextFs, err := newRefFS(ctx, gw, contextInput, llb.WithCaps(caps))
	// if err != nil {
	// 	return nil, err
	// }

	pool := &runtimes.Pool{}

	kitdruntime, err := runtimes.NewBuildkitFrontend(gw, inputs, runtimes.BuildkitConfig{
		CertsDir: certsDir,
	})
	if err != nil {
		return nil, err
	}

	pool.Runtimes = append(pool.Runtimes, runtimes.Assoc{
		Runtime:  kitdruntime,
		Platform: bass.LinuxPlatform,
	})

	ctx = bass.WithRuntimePool(ctx, pool)

	args := filter(opts, buildArgPrefix)
	env := bass.NewEmptyScope()
	for k, v := range args {
		env.Set(bass.Symbol(k), bass.String(v))
	}

	runSt := bass.RunState{
		Env: env,
		// directly pass the context by local name, masquerading it as the host path
		Dir:    bass.NewHostDir(localNameContext),
		Stdin:  bass.Stdin,
		Stdout: bass.Stdout,
	}

	module := bass.NewRunScope(bass.Ground, runSt)

	val, err := bass.EvalFSFile(ctx, module, bass.NewFSPath(scriptFs, bass.ParseFileOrDirPath(scriptFn)))
	if err != nil {
		return nil, err
	}

	var thunk bass.Thunk
	if err := val.Decode(&thunk); err != nil {
		return nil, err
	}

	platform := ocispecs.Platform{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	builder := kitdruntime.NewBuilder(gw)

	ib, err := builder.Build(
		ctx,
		thunk,
		false, // don't run any entrypoint
	)
	if err != nil {
		return nil, err
	}

	def, err := ib.FS.Marshal(ctx, llb.WithCaps(caps))
	if err != nil {
		return nil, err
	}

	outRes, err := gw.Solve(ctx, gwclient.SolveRequest{
		Definition: def.ToPB(),
	})
	if err != nil {
		return nil, err
	}

	if _, hasConfig := outRes.Metadata[exptypes.ExporterImageConfigKey]; !hasConfig {
		configBytes, err := json.Marshal(ocispecs.Image{
			Architecture: platform.Architecture,
			OS:           platform.OS,
			OSVersion:    platform.OSVersion,
			OSFeatures:   platform.OSFeatures,
			Config:       ib.Config,
		})
		if err != nil {
			return nil, err
		}

		outRes.AddMeta(exptypes.ExporterImageConfigKey, configBytes)
	}

	return outRes, nil
}

func filter(opt map[string]string, key string) map[string]string {
	m := map[string]string{}
	for k, v := range opt {
		if strings.HasPrefix(k, key) {
			m[strings.TrimPrefix(k, key)] = v
		}
	}
	return m
}
