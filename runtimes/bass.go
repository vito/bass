package runtimes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/vito/bass"
	"github.com/vito/bass/std"
	"github.com/vito/bass/zapctx"
	"go.uber.org/zap"
)

// Ext is the canonical file extension for Bass source code.
const Ext = ".bass"

type Bass struct {
	Pool *Pool

	responses map[string][]byte
	envs      map[string]*bass.Env
	mutex     sync.Mutex
}

var _ Runtime = &Bass{}

func NewBass(pool *Pool) Runtime {
	return &Bass{
		Pool: pool,

		responses: map[string][]byte{},
		envs:      map[string]*bass.Env{},
	}
}

func (runtime *Bass) Run(ctx context.Context, workload bass.Workload) error {
	logger := zapctx.FromContext(ctx)

	key, err := workload.SHA1()
	if err != nil {
		return err
	}

	// TODO: per-key lock around full runtime to handle concurrent loading (if
	// that ever comes up)
	runtime.mutex.Lock()
	_, cached := runtime.responses[key]
	runtime.mutex.Unlock()

	if cached {
		logger.Debug("cached", zap.Any("workload", workload))
		// cached
		return nil
	}

	logger.Info("loading", zap.Any("workload", workload))

	response := new(bytes.Buffer)
	state := RunState{
		Dir:    nil, // set below
		Args:   bass.NewList(workload.Args...),
		Stdout: bass.NewSink(bass.NewJSONSink(workload.String(), response)),
		Stdin:  bass.NewSource(bass.NewStaticSource(workload.Stdin...)),
	}

	var env *bass.Env

	if workload.Path.Cmd != nil {
		cp := workload.Path.Cmd
		state.Dir = bass.NewFSDir(std.FS)

		env = NewEnv(runtime.Pool, state)

		_, err := bass.EvalFSFile(ctx, env, std.FS, cp.Command+Ext)
		if err != nil {
			return err
		}
	} else if workload.Path.Host != nil {
		hostp := workload.Path.Host

		state.Dir = bass.HostPath{
			Path: filepath.Dir(hostp.Path),
		}

		env = NewEnv(runtime.Pool, state)

		_, err := bass.EvalFile(ctx, env, hostp.Path+Ext)
		if err != nil {
			return err
		}
	} else if workload.Path.WorkloadFile != nil {
		wlp := workload.Path.WorkloadFile

		// TODO: this is hokey
		dir := *wlp
		dirp := wlp.Path.File.Dir()
		dir.Path = bass.FileOrDirPath{Dir: &dirp}
		state.Dir = dir
		env = NewEnv(runtime.Pool, state)

		src := new(bytes.Buffer)
		err := runtime.Pool.Export(ctx, src, wlp.Workload, bass.FilePath{Path: wlp.Path.File.Path + Ext})
		if err != nil {
			return err
		}

		_, err = bass.EvalReader(ctx, env, src, wlp.String())
		if err != nil {
			return err
		}
	} else if workload.Path.FS != nil {
		fsp := workload.Path.FS

		dir := fsp.Path.File.Dir()
		state.Dir = bass.FSPath{
			FS:   fsp.FS,
			Path: bass.FileOrDirPath{Dir: &dir},
		}

		env = NewEnv(runtime.Pool, state)

		_, err := bass.EvalFSFile(ctx, env, workload.Path.FS.FS, fsp.Path.String()+Ext)
		if err != nil {
			return err
		}
	} else if workload.Path.File != nil {
		// TODO: better error
		return fmt.Errorf("bad path: did you mean *dir*/%s? (. is only resolveable in a container)", workload.Path.File)
	} else {
		val := workload.Path.ToValue()
		return fmt.Errorf("impossible: unknown workload path type %T: %s", val, val)
	}

	runtime.mutex.Lock()
	runtime.envs[key] = env
	runtime.responses[key] = response.Bytes()
	runtime.mutex.Unlock()

	return nil
}

func (runtime *Bass) Response(ctx context.Context, w io.Writer, workload bass.Workload) error {
	key, err := workload.SHA1()
	if err != nil {
		return err
	}

	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()

	res, found := runtime.responses[key]
	if found {
		// XXX: this is a little strange since the other end just unmarshals it,
		// but let's roll with it for now so we don't have to rehash the runtime
		// interface
		//
		// the runtime interface just takes an io.Writer in case someday we want to
		// handle direct responses (not JSON streams) - worth reconsidering at some
		// point so this can just return a StaticSource
		_, err := w.Write(res)
		return err
	}

	// TODO: Bass always calls .Run before .Response so it should always be
	// there, but the interface should probably handle that condition better
	//
	// maybe re-run it? or just get rid of Run?
	return fmt.Errorf("response not found")
}

func (runtime *Bass) Env(ctx context.Context, workload bass.Workload) (*bass.Env, error) {
	key, err := workload.SHA1()
	if err != nil {
		return nil, err
	}

	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()

	res, found := runtime.envs[key]
	if found {
		return res, nil
	}

	// TODO: Bass always calls .Run before .Env so it should always be there, but
	// the interface should probably handle that condition better
	//
	// maybe re-run it? or just get rid of Run?
	return nil, fmt.Errorf("env not found")
}

func (runtime *Bass) Export(context.Context, io.Writer, bass.Workload, bass.FilesystemPath) error {
	return fmt.Errorf("cannot export from native workload")
}
