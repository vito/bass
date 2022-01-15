package runtimes

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
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
const NoExt = ""

type Bass struct {
	External bass.RuntimePool

	responses map[string][]byte
	modules   map[string]*bass.Scope
	mutex     sync.Mutex
}

var _ bass.Runtime = &Bass{}

func NewBass(pool bass.RuntimePool) bass.Runtime {
	return &Bass{
		External: pool,

		responses: map[string][]byte{},
		modules:   map[string]*bass.Scope{},
	}
}

func (runtime *Bass) Prune(ctx context.Context, opts bass.PruneOpts) error {
	return nil
}

func (runtime *Bass) Resolve(ctx context.Context, ref bass.ThunkImageRef) (bass.ThunkImageRef, error) {
	return bass.ThunkImageRef{}, errors.New("bass runtime cannot resolve images")
}

func (runtime *Bass) Run(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	_, response, err := runtime.run(ctx, thunk, NoExt)
	if err != nil {
		return err
	}

	_, err = w.Write(response)
	if err != nil {
		return err
	}

	return nil
}

func (runtime *Bass) run(ctx context.Context, thunk bass.Thunk, ext string) (*bass.Scope, []byte, error) {
	logger := zapctx.FromContext(ctx)

	key, err := thunk.SHA1()
	if err != nil {
		return nil, nil, err
	}

	logger = logger.With(
		zap.String("thunk", key),
		zap.String("path", thunk.Cmd.ToValue().String()),
	)

	// TODO: per-key lock around full runtime to handle concurrent loading (if
	// that ever comes up)
	runtime.mutex.Lock()
	module, cached := runtime.modules[key]
	response, cached := runtime.responses[key]
	runtime.mutex.Unlock()

	if cached {
		logger.Debug("already loaded thunk")
		return module, response, nil
	}

	responseBuf := new(bytes.Buffer)
	state := RunState{
		Dir:    nil, // set below
		Args:   bass.NewList(thunk.Args...),
		Stdout: bass.NewSink(bass.NewJSONSink(thunk.String(), responseBuf)),
		Stdin:  bass.NewSource(bass.NewInMemorySource(thunk.Stdin...)),
	}

	if thunk.Cmd.Cmd != nil {
		cp := thunk.Cmd.Cmd
		state.Dir = bass.NewFSDir(std.FS)

		module = NewScope(bass.NewEmptyScope(bass.NewStandardScope(), internal.Scope), state)

		_, err := bass.EvalFSFile(ctx, module, std.FS, cp.Command+ext)
		if err != nil {
			return nil, nil, err
		}
	} else if thunk.Cmd.Host != nil {
		hostp := thunk.Cmd.Host

		fp := filepath.Join(hostp.FromSlash() + ext)
		abs, err := filepath.Abs(filepath.Dir(fp))
		if err != nil {
			return nil, nil, err
		}

		state.Dir = bass.NewHostPath(abs)

		module = NewScope(bass.NewStandardScope(), state)

		_, err = bass.EvalFile(ctx, module, fp)
		if err != nil {
			return nil, nil, err
		}
	} else if thunk.Cmd.ThunkFile != nil {
		wlp := thunk.Cmd.ThunkFile

		// TODO: this is hokey
		dir := *wlp
		dirp := wlp.Path.File.Dir()
		dir.Path = bass.FileOrDirPath{Dir: &dirp}
		state.Dir = dir
		module = NewScope(bass.Ground, state)

		fp := bass.FilePath{Path: wlp.Path.File.Path + ext}
		src := new(bytes.Buffer)

		runt, err := runtime.External.Select(wlp.Thunk.Platform())
		if err != nil {
			return nil, nil, err
		}

		err = runt.ExportPath(ctx, src, bass.ThunkPath{
			Thunk: wlp.Thunk,
			Path:  fp.FileOrDir(),
		})
		if err != nil {
			return nil, nil, err
		}

		tr := tar.NewReader(src)

		_, err = tr.Next()
		if err != nil {
			return nil, nil, fmt.Errorf("export %s: %w", fp, err)
		}

		_, err = bass.EvalReader(ctx, module, tr, wlp.String())
		if err != nil {
			return nil, nil, err
		}
	} else if thunk.Cmd.FS != nil {
		fsp := thunk.Cmd.FS

		dir := fsp.Path.File.Dir()
		state.Dir = bass.FSPath{
			FS:   fsp.FS,
			Path: bass.FileOrDirPath{Dir: &dir},
		}

		module = NewScope(bass.Ground, state)

		_, err := bass.EvalFSFile(ctx, module, thunk.Cmd.FS.FS, fsp.Path.String()+ext)
		if err != nil {
			return nil, nil, err
		}
	} else if thunk.Cmd.File != nil {
		// TODO: better error
		return nil, nil, fmt.Errorf("bad path: did you mean *dir*/%s? (. is only resolveable in a container)", thunk.Cmd.File)
	} else {
		val := thunk.Cmd.ToValue()
		return nil, nil, fmt.Errorf("impossible: unknown thunk path type %T: %s", val, val)
	}

	response = responseBuf.Bytes()

	module.Name = thunk.String()

	runtime.mutex.Lock()
	runtime.modules[key] = module
	runtime.responses[key] = response
	runtime.mutex.Unlock()

	return module, response, nil
}

func (runtime *Bass) Response(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	_, response, err := runtime.run(ctx, thunk, NoExt)
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

func (runtime *Bass) Load(ctx context.Context, thunk bass.Thunk) (*bass.Scope, error) {
	module, _, err := runtime.run(ctx, thunk, Ext)
	if err != nil {
		return nil, err
	}

	return module, nil
}

func (runtime *Bass) Export(context.Context, io.Writer, bass.Thunk) error {
	return fmt.Errorf("cannot export from bass thunk")
}

func (runtime *Bass) ExportPath(context.Context, io.Writer, bass.ThunkPath) error {
	return fmt.Errorf("cannot export from bass thunk")
}
