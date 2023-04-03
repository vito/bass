package bass

import (
	"context"
	"errors"
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

	if len(thunk.Args) == 0 {
		return nil, errors.New("Bass thunk has no command")
	}

	cmd := thunk.Args[0]

	var ok bool

	var cmdp CommandPath
	if cmd.Decode(&cmdp) == nil {
		ok = true

		state.Dir = NewFSDir(std.FS)

		module = NewRunScope(NewEmptyScope(session.Root, Internal), state)

		source := NewFSPath(
			std.FS,
			ParseFileOrDirPath(cmdp.Command+Ext),
		)

		_, err := EvalFSFile(ctx, module, source)
		if err != nil {
			return nil, err
		}
	}

	var hostp HostPath
	if cmd.Decode(&hostp) == nil {
		ok = true

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
	}

	var thunkp ThunkPath
	if cmd.Decode(&thunkp) == nil {
		ok = true

		source := ThunkPath{
			Thunk: thunkp.Thunk,
			Path:  FilePath{Path: thunkp.Path.File.Path}.FileOrDir(),
		}

		modFile, err := source.CachePath(ctx, CacheHome)
		if err != nil {
			return nil, err
		}

		state.Dir = thunkp.Dir()

		module = NewRunScope(session.Root, state)

		_, err = EvalFile(ctx, module, modFile, source)
		if err != nil {
			return nil, err
		}
	}

	var fsp *FSPath
	if cmd.Decode(&fsp) == nil {
		ok = true

		dir := fsp.Path.File.Dir()
		state.Dir = NewFSPath(fsp.FS, FileOrDirPath{Dir: &dir})

		module = NewRunScope(session.Root, state)

		_, err := EvalFSFile(ctx, module, fsp)
		if err != nil {
			return nil, err
		}
	}

	var filep FilePath
	if cmd.Decode(&filep) == nil {
		// TODO: better error
		return nil, fmt.Errorf("bad path: did you mean *dir*/%s? (. is only resolveable in a container)", filep.Path)
	}

	if !ok {
		return nil, fmt.Errorf("impossible: unknown thunk path type %T: %s", cmd, cmd)
	}

	if runMain {
		err := RunMain(ctx, module, thunk.Args[1:]...)
		if err != nil {
			return nil, err
		}
	}

	module.Name = thunk.String()

	return module, nil
}
