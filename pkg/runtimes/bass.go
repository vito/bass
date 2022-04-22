package runtimes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/internal"
	"github.com/vito/bass/std"
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
	_, err := runtime.run(ctx, w, thunk, true)
	if err != nil {
		return err
	}

	return nil
}

func (runtime *Bass) Load(ctx context.Context, thunk bass.Thunk) (*bass.Scope, error) {
	key, err := thunk.SHA256()
	if err != nil {
		return nil, err
	}

	// TODO: per-key lock around full runtime to handle concurrent loading (if
	// that ever comes up)
	runtime.mutex.Lock()
	module, cached := runtime.modules[key]
	runtime.mutex.Unlock()

	if cached {
		return module, nil
	}

	module, err = runtime.run(ctx, io.Discard, thunk, false)
	if err != nil {
		return nil, err
	}

	runtime.mutex.Lock()
	runtime.modules[key] = module
	runtime.mutex.Unlock()

	return module, nil
}

func (runtime *Bass) Export(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	return fmt.Errorf("export %s: cannot export bass thunk", thunk)
}

func (runtime *Bass) ExportPath(ctx context.Context, w io.Writer, path bass.ThunkPath) error {
	return fmt.Errorf("export %s: cannot export path from bass thunk", path)
}

func (runtime *Bass) run(ctx context.Context, w io.Writer, thunk bass.Thunk, runMain bool) (*bass.Scope, error) {
	var ext string
	if runMain {
		ext = NoExt
	} else {
		ext = Ext
	}

	state := RunState{
		Dir:    nil, // set below
		Stdout: bass.NewSink(bass.NewJSONSink(thunk.String(), w)),
		Stdin:  bass.NewSource(bass.NewInMemorySource(thunk.Stdin...)),
		Env:    thunk.Env,
	}

	var module *bass.Scope

	if thunk.Cmd.Cmd != nil {
		cp := thunk.Cmd.Cmd
		state.Dir = bass.NewFSDir(std.FSID, std.FS)

		module = NewScope(bass.NewEmptyScope(bass.NewStandardScope(), internal.Scope), state)

		_, err := bass.EvalFSFile(ctx, module, std.FS, cp.Command+ext)
		if err != nil {
			return nil, err
		}
	} else if thunk.Cmd.Host != nil {
		hostp := thunk.Cmd.Host

		fp := filepath.Join(hostp.FromSlash() + ext)
		abs, err := filepath.Abs(filepath.Dir(fp))
		if err != nil {
			return nil, err
		}

		state.Dir = bass.NewHostDir(abs)

		module = NewScope(bass.NewStandardScope(), state)

		_, err = bass.EvalFile(ctx, module, fp)
		if err != nil {
			return nil, err
		}
	} else if thunk.Cmd.ThunkFile != nil {
		modFile, err := bass.CacheThunkPath(ctx, bass.ThunkPath{
			Thunk: thunk.Cmd.ThunkFile.Thunk,
			Path:  bass.FilePath{Path: thunk.Cmd.ThunkFile.Path.File.Path + ext}.FileOrDir(),
		})
		if err != nil {
			return nil, err
		}

		state.Dir = thunk.Cmd.ThunkFile.Dir()

		module = NewScope(bass.Ground, state)

		_, err = bass.EvalFile(ctx, module, modFile)
		if err != nil {
			return nil, err
		}
	} else if thunk.Cmd.FS != nil {
		fsp := thunk.Cmd.FS

		dir := fsp.Path.File.Dir()
		state.Dir = bass.FSPath{
			ID:   fsp.ID,
			FS:   fsp.FS,
			Path: bass.FileOrDirPath{Dir: &dir},
		}

		module = NewScope(bass.Ground, state)

		_, err := bass.EvalFSFile(ctx, module, thunk.Cmd.FS.FS, fsp.Path.String()+ext)
		if err != nil {
			return nil, err
		}
	} else if thunk.Cmd.File != nil {
		// TODO: better error
		return nil, fmt.Errorf("bad path: did you mean *dir*/%s? (. is only resolveable in a container)", thunk.Cmd.File)
	} else {
		val := thunk.Cmd.ToValue()
		return nil, fmt.Errorf("impossible: unknown thunk path type %T: %s", val, val)
	}

	if runMain {
		err := bass.RunMain(ctx, module, thunk.Args...)
		if err != nil {
			return nil, err
		}
	}

	module.Name = thunk.String()

	return module, nil
}
