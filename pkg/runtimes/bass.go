package runtimes

import (
	"bytes"
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
	_, response, err := runtime.run(ctx, thunk, true)
	if err != nil {
		return err
	}

	_, err = w.Write(response)
	if err != nil {
		return err
	}

	return nil
}

func (runtime *Bass) Response(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	_, response, err := runtime.run(ctx, thunk, true)
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
	module, _, err := runtime.run(ctx, thunk, false)
	if err != nil {
		return nil, err
	}

	return module, nil
}

func (runtime *Bass) Export(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	return fmt.Errorf("export %s: cannot export bass thunk", thunk)
}

func (runtime *Bass) ExportPath(ctx context.Context, w io.Writer, path bass.ThunkPath) error {
	return fmt.Errorf("export %s: cannot export path from bass thunk", path)
}

func (runtime *Bass) Close() error {
	return nil
}

func (runtime *Bass) run(ctx context.Context, thunk bass.Thunk, runMain bool) (*bass.Scope, []byte, error) {
	var ext string
	if runMain {
		ext = NoExt
	} else {
		ext = Ext
	}

	key, err := thunk.SHA256()
	if err != nil {
		return nil, nil, err
	}

	// TODO: per-key lock around full runtime to handle concurrent loading (if
	// that ever comes up)
	runtime.mutex.Lock()
	module, cached := runtime.modules[key]
	response, cached := runtime.responses[key]
	runtime.mutex.Unlock()

	if cached {
		return module, response, nil
	}

	responseBuf := new(bytes.Buffer)
	state := bass.RunState{
		Dir:    nil, // set below
		Stdout: bass.NewSink(bass.NewJSONSink(thunk.String(), responseBuf)),
		Stdin:  bass.NewSource(bass.NewInMemorySource(thunk.Stdin...)),
		Env:    thunk.Env,
	}

	if thunk.Cmd.Cmd != nil {
		cp := thunk.Cmd.Cmd
		state.Dir = bass.NewFSDir(std.FSID, std.FS)

		module = bass.NewRunScope(bass.NewEmptyScope(bass.NewStandardScope(), internal.Scope), state)

		source := bass.NewFSPath(
			std.FSID,
			std.FS,
			bass.ParseFileOrDirPath(cp.Command+ext),
		)

		_, err := bass.EvalFSFile(ctx, module, source)
		if err != nil {
			return nil, nil, err
		}
	} else if thunk.Cmd.Host != nil {
		hostp := *thunk.Cmd.Host

		fp := filepath.Join(hostp.FromSlash() + ext)
		abs, err := filepath.Abs(filepath.Dir(fp))
		if err != nil {
			return nil, nil, err
		}

		state.Dir = bass.NewHostDir(abs)

		module = bass.NewRunScope(bass.NewStandardScope(), state)

		withExt := hostp
		withExt.Path = bass.ParseFileOrDirPath(hostp.Path.String() + ext)

		_, err = bass.EvalFile(ctx, module, fp, withExt)
		if err != nil {
			return nil, nil, err
		}
	} else if thunk.Cmd.ThunkFile != nil {
		source := bass.ThunkPath{
			Thunk: thunk.Cmd.ThunkFile.Thunk,
			Path:  bass.FilePath{Path: thunk.Cmd.ThunkFile.Path.File.Path + ext}.FileOrDir(),
		}

		modFile, err := source.CachePath(ctx, bass.CacheHome)
		if err != nil {
			return nil, nil, err
		}

		state.Dir = thunk.Cmd.ThunkFile.Dir()

		module = bass.NewRunScope(bass.Ground, state)

		_, err = bass.EvalFile(ctx, module, modFile, source)
		if err != nil {
			return nil, nil, err
		}
	} else if thunk.Cmd.FS != nil {
		fsp := thunk.Cmd.FS

		dir := fsp.Path.File.Dir()
		state.Dir = bass.FSPath{
			ID:   fsp.ID,
			FS:   fsp.FS,
			Path: bass.FileOrDirPath{Dir: &dir},
		}

		module = bass.NewRunScope(bass.Ground, state)

		withExt := *fsp
		withExt.Path = bass.ParseFileOrDirPath(fsp.Path.String() + ext)

		_, err := bass.EvalFSFile(ctx, module, withExt)
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

	if runMain {
		err = bass.RunMain(ctx, module, thunk.Args...)
		if err != nil {
			return nil, nil, err
		}
	}

	response = responseBuf.Bytes()

	module.Name = thunk.String()

	runtime.mutex.Lock()
	runtime.modules[key] = module
	runtime.responses[key] = response
	runtime.mutex.Unlock()

	return module, response, nil
}
