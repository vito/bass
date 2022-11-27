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

type Session struct {
	// Root is the base level scope inherited by all modules.
	Root *Scope

	modules map[uint64]*Scope
	mutex   sync.Mutex
}

// NewBass returns a new session with Ground as its root scope.
func NewBass() *Session {
	return NewSession(Ground)
}

// NewSession returns a new session with the specified root scope.
func NewSession(ground *Scope) *Session {
	return &Session{
		Root:    ground,
		modules: map[uint64]*Scope{},
	}
}

func (session *Session) Run(ctx context.Context, thunk Thunk, state RunState) error {
	_, err := session.run(ctx, thunk, state, true)
	if err != nil {
		return err
	}

	return nil
}

func (session *Session) Load(ctx context.Context, thunk Thunk) (*Scope, error) {
	key, err := thunk.HashKey()
	if err != nil {
		return nil, err
	}

	session.mutex.Lock()
	module, cached := session.modules[key]
	session.mutex.Unlock()

	if cached {
		return module, nil
	}

	module, err = session.run(ctx, thunk, thunk.RunState(io.Discard), false)
	if err != nil {
		return nil, err
	}

	session.mutex.Lock()
	session.modules[key] = module
	session.mutex.Unlock()

	return module, nil
}

func (session *Session) run(ctx context.Context, thunk Thunk, state RunState, runMain bool) (*Scope, error) {
	custodian := NewCustodian()
	defer custodian.Close()

	ctx = WithCustodian(ctx, custodian)

	var module *Scope

	if thunk.Cmd.Cmd != nil {
		cp := thunk.Cmd.Cmd
		state.Dir = NewFSDir(std.FS)

		module = NewRunScope(NewEmptyScope(session.Root, Internal), state)

		source := NewFSPath(
			std.FS,
			ParseFileOrDirPath(cp.Command+Ext),
		)

		_, err := EvalFSFile(ctx, module, source)
		if err != nil {
			return nil, err
		}
	} else if thunk.Cmd.Host != nil {
		hostp := *thunk.Cmd.Host

		fp := filepath.Join(hostp.FromSlash())
		abs, err := filepath.Abs(filepath.Dir(fp))
		if err != nil {
			return nil, err
		}

		state.Dir = NewHostDir(abs)

		module = NewRunScope(session.Root, state)

		_, err = EvalFile(ctx, module, fp, hostp)
		if err != nil {
			return nil, err
		}
	} else if thunk.Cmd.Thunk != nil {
		source := ThunkPath{
			Thunk: thunk.Cmd.Thunk.Thunk,
			Path:  FilePath{Path: thunk.Cmd.Thunk.Path.File.Path}.FileOrDir(),
		}

		modFile, err := source.CachePath(ctx, CacheHome)
		if err != nil {
			return nil, err
		}

		state.Dir = thunk.Cmd.Thunk.Dir()

		module = NewRunScope(session.Root, state)

		_, err = EvalFile(ctx, module, modFile, source)
		if err != nil {
			return nil, err
		}
	} else if thunk.Cmd.FS != nil {
		fsp := thunk.Cmd.FS

		dir := fsp.Path.File.Dir()
		state.Dir = &FSPath{
			FS:   fsp.FS,
			Path: FileOrDirPath{Dir: &dir},
		}

		module = NewRunScope(session.Root, state)

		_, err := EvalFSFile(ctx, module, fsp)
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
