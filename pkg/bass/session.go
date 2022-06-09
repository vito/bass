package bass

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/vito/bass/std"
)

// Ext is the canonical file extension for Bass source code.
const Ext = ".bass"
const NoExt = ""

type Session struct {
	modules map[string]*Scope
	mutex   sync.Mutex
}

func NewBass() *Session {
	return &Session{
		modules: map[string]*Scope{},
	}
}

func (runtime *Session) Run(ctx context.Context, w io.Writer, thunk Thunk) error {
	_, err := runtime.run(ctx, thunk, true, w)
	if err != nil {
		return err
	}

	return nil
}

func (runtime *Session) Load(ctx context.Context, thunk Thunk) (*Scope, error) {
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

	module, err = runtime.run(ctx, thunk, false, io.Discard)
	if err != nil {
		return nil, err
	}

	runtime.mutex.Lock()
	runtime.modules[key] = module
	runtime.mutex.Unlock()

	return module, nil
}

func (runtime *Session) run(ctx context.Context, thunk Thunk, runMain bool, w io.Writer) (*Scope, error) {
	var ext string
	if runMain {
		ext = NoExt
	} else {
		ext = Ext
	}

	var module *Scope

	state := RunState{
		Dir:    nil, // set below
		Stdout: NewSink(NewJSONSink(thunk.String(), w)),
		Stdin:  NewSource(NewInMemorySource(thunk.Stdin...)),
		Env:    thunk.Env,
	}

	if thunk.Cmd.Cmd != nil {
		cp := thunk.Cmd.Cmd
		state.Dir = NewFSDir(std.FSID, std.FS)

		module = NewRunScope(NewEmptyScope(NewStandardScope(), Internal), state)

		source := NewFSPath(
			std.FSID,
			std.FS,
			ParseFileOrDirPath(cp.Command+ext),
		)

		_, err := EvalFSFile(ctx, module, source)
		if err != nil {
			return nil, err
		}
	} else if thunk.Cmd.Host != nil {
		hostp := *thunk.Cmd.Host

		fp := filepath.Join(hostp.FromSlash() + ext)
		abs, err := filepath.Abs(filepath.Dir(fp))
		if err != nil {
			return nil, err
		}

		state.Dir = NewHostDir(abs)

		module = NewRunScope(NewStandardScope(), state)

		withExt := hostp
		withExt.Path = ParseFileOrDirPath(hostp.Path.Slash() + ext)

		_, err = EvalFile(ctx, module, fp, withExt)
		if err != nil {
			return nil, err
		}
	} else if thunk.Cmd.Thunk != nil {
		source := ThunkPath{
			Thunk: thunk.Cmd.Thunk.Thunk,
			Path:  FilePath{Path: thunk.Cmd.Thunk.Path.File.Path + ext}.FileOrDir(),
		}

		modFile, err := source.CachePath(ctx, CacheHome)
		if err != nil {
			return nil, err
		}

		state.Dir = thunk.Cmd.Thunk.Dir()

		module = NewRunScope(Ground, state)

		_, err = EvalFile(ctx, module, modFile, source)
		if err != nil {
			return nil, err
		}
	} else if thunk.Cmd.FS != nil {
		fsp := thunk.Cmd.FS

		dir := fsp.Path.File.Dir()
		state.Dir = FSPath{
			ID:   fsp.ID,
			FS:   fsp.FS,
			Path: FileOrDirPath{Dir: &dir},
		}

		module = NewRunScope(Ground, state)

		withExt := *fsp
		withExt.Path = ParseFileOrDirPath(fsp.Path.Slash() + ext)

		_, err := EvalFSFile(ctx, module, withExt)
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
		err := RunMain(ctx, module, thunk.Args...)
		if err != nil {
			return nil, err
		}
	}

	module.Name = thunk.String()

	return module, nil
}
