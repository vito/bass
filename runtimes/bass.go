package runtimes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/vito/bass"
	"github.com/vito/bass/internal"
	"github.com/vito/bass/std"
	"github.com/vito/bass/zapctx"
	"go.uber.org/zap"
)

// Ext is the canonical file extension for Bass source code.
const Ext = ".bass"

type Bass struct {
	External *Pool

	responses map[string][]byte
	modules   map[string]*bass.Env
	mutex     sync.Mutex
}

var _ bass.Runtime = &Bass{}

func NewBass(pool *Pool) bass.Runtime {
	return &Bass{
		External: pool,

		responses: map[string][]byte{},
		modules:   map[string]*bass.Env{},
	}
}

func (runtime *Bass) Run(ctx context.Context, w io.Writer, workload bass.Workload) error {
	_, response, err := runtime.run(ctx, workload)
	if err != nil {
		return err
	}

	_, err = w.Write(response)
	if err != nil {
		return err
	}

	return nil
}

func (runtime *Bass) run(ctx context.Context, workload bass.Workload) (*bass.Env, []byte, error) {
	logger := zapctx.FromContext(ctx)

	key, err := workload.SHA1()
	if err != nil {
		return nil, nil, err
	}

	logger = logger.With(
		zap.String("workload", key),
		zap.String("path", workload.Path.ToValue().String()),
	)

	// TODO: per-key lock around full runtime to handle concurrent loading (if
	// that ever comes up)
	runtime.mutex.Lock()
	module, cached := runtime.modules[key]
	response, cached := runtime.responses[key]
	runtime.mutex.Unlock()

	if cached {
		logger.Debug("already loaded workload")
		return module, response, nil
	}

	logger.Debug("loading workload")

	responseBuf := new(bytes.Buffer)
	state := RunState{
		Dir:    nil, // set below
		Args:   bass.NewList(workload.Args...),
		Stdout: bass.NewSink(bass.NewJSONSink(workload.String(), responseBuf)),
		Stdin:  bass.NewSource(bass.NewInMemorySource(workload.Stdin...)),
	}

	if workload.Path.Cmd != nil {
		cp := workload.Path.Cmd
		state.Dir = bass.NewFSDir(std.FS)

		module = NewEnv(bass.NewEnv(bass.NewStandardEnv(), internal.Env), state)

		_, err := bass.EvalFSFile(ctx, module, std.FS, cp.Command+Ext)
		if err != nil {
			return nil, nil, err
		}
	} else if workload.Path.Host != nil {
		hostp := workload.Path.Host

		state.Dir = bass.HostPath{
			Path: filepath.Dir(hostp.Path),
		}

		module = NewEnv(bass.NewEnv(bass.Ground, internal.Env), state)

		_, err := bass.EvalFile(ctx, module, hostp.Path+Ext)
		if err != nil {
			return nil, nil, err
		}
	} else if workload.Path.WorkloadFile != nil {
		wlp := workload.Path.WorkloadFile

		// TODO: this is hokey
		dir := *wlp
		dirp := wlp.Path.File.Dir()
		dir.Path = bass.FileOrDirPath{Dir: &dirp}
		state.Dir = dir
		module = NewEnv(bass.Ground, state)

		src := new(bytes.Buffer)
		err := runtime.External.Export(ctx, src, wlp.Workload, bass.FilePath{Path: wlp.Path.File.Path + Ext})
		if err != nil {
			return nil, nil, err
		}

		_, err = bass.EvalReader(ctx, module, src, wlp.String())
		if err != nil {
			return nil, nil, err
		}
	} else if workload.Path.FS != nil {
		fsp := workload.Path.FS

		dir := fsp.Path.File.Dir()
		state.Dir = bass.FSPath{
			FS:   fsp.FS,
			Path: bass.FileOrDirPath{Dir: &dir},
		}

		module = NewEnv(bass.Ground, state)

		_, err := bass.EvalFSFile(ctx, module, workload.Path.FS.FS, fsp.Path.String()+Ext)
		if err != nil {
			return nil, nil, err
		}
	} else if workload.Path.File != nil {
		// TODO: better error
		return nil, nil, fmt.Errorf("bad path: did you mean *dir*/%s? (. is only resolveable in a container)", workload.Path.File)
	} else {
		val := workload.Path.ToValue()
		return nil, nil, fmt.Errorf("impossible: unknown workload path type %T: %s", val, val)
	}

	response = responseBuf.Bytes()

	runtime.mutex.Lock()
	runtime.modules[key] = module
	runtime.responses[key] = response
	runtime.mutex.Unlock()

	return module, response, nil
}

func (runtime *Bass) Response(ctx context.Context, w io.Writer, workload bass.Workload) error {
	_, response, err := runtime.run(ctx, workload)
	if err != nil {
		return err
	}

	// XXX: this is a little strange since the other end just unmarshals it,
	// but let's roll with it for now so we don't have to rehash the runtime
	// interface
	//
	// the runtime interface just takes an io.Writer in case someday we want to
	// handle direct responses (not JSON streams) - worth reconsidering at some
	// point so this can just return an InMemorySource
	_, err = w.Write(response)
	return err
}

func (runtime *Bass) Load(ctx context.Context, workload bass.Workload) (*bass.Env, error) {
	module, _, err := runtime.run(ctx, workload)
	if err != nil {
		return nil, err
	}

	return module, nil
}

func (runtime *Bass) Export(context.Context, io.Writer, bass.Workload, bass.FilesystemPath) error {
	return fmt.Errorf("cannot export from bass workload")
}
