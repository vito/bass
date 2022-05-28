package bass

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// HostPath is a Path representing an absolute path on the host machine's
// filesystem.
type HostPath struct {
	ContextDir string        `json:"context"`
	Path       FileOrDirPath `json:"path"`
}

func (hp HostPath) FromSlash() string {
	return filepath.Join(hp.ContextDir, hp.Path.FilesystemPath().FromSlash())
}

var _ Value = HostPath{}

func NewHostDir(contextDir string) HostPath {
	return NewHostPath(contextDir, ParseFileOrDirPath("."))
}

func NewHostPath(contextDir string, path FileOrDirPath) HostPath {
	return HostPath{
		ContextDir: contextDir,
		Path:       path,
	}
}

func ParseHostPath(path string) HostPath {
	return NewHostPath(
		filepath.Dir(path),
		ParseFileOrDirPath(filepath.Base(path)),
	)
}

func (value HostPath) String() string {
	return fmt.Sprintf("<host: %s>", value.fpath())
}

func (value HostPath) Equal(other Value) bool {
	var o HostPath
	return other.Decode(&o) == nil &&
		value.ContextDir == o.ContextDir &&
		value.Path.FilesystemPath().Equal(o.Path.FilesystemPath())
}

func (value HostPath) Decode(dest any) error {
	switch x := dest.(type) {
	case *HostPath:
		*x = value
		return nil
	case *Path:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Applicative:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	case *Readable:
		*x = value
		return nil
	case Decodable:
		return x.FromValue(value)
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value *HostPath) FromValue(val Value) error {
	var obj *Scope
	if err := val.Decode(&obj); err != nil {
		return fmt.Errorf("%T.FromValue: %w", value, err)
	}

	return decodeStruct(obj, value)
}

// Eval returns the value.
func (value HostPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = ThunkPath{}

func (app HostPath) Unwrap() Combiner {
	if app.Path.File != nil {
		return ThunkOperative{
			Cmd: ThunkCmd{
				Host: &app,
			},
		}
	} else {
		return ExtendOperative{app}
	}
}

var _ Combiner = HostPath{}

func (combiner HostPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(combiner.Unwrap()).Call(ctx, val, scope, cont)
}

var _ Path = HostPath{}

func (path HostPath) Name() string {
	return filepath.Base(path.fpath())
}

func (path HostPath) Extend(ext Path) (Path, error) {
	extended := path

	var err error
	extended.Path, err = path.Path.Extend(ext)
	if err != nil {
		return nil, err
	}

	return extended, nil
}

// TODO: is this safe?
var _ Readable = HostPath{}

func (path HostPath) checkEscape() (string, error) {
	r := filepath.Clean(path.fpath())
	c := filepath.Clean(path.ContextDir)

	if r == c {
		return r, nil
	}

	c += "/"

	if strings.HasPrefix(r, c) {
		return r, nil
	}

	return "", HostPathEscapeError{
		ContextDir: path.ContextDir,
		Attempted:  path.Path,
	}
}

func (path HostPath) CachePath(_ctx context.Context, _dest string) (string, error) {
	return path.checkEscape()
}

func (path HostPath) Open(context.Context) (io.ReadCloser, error) {
	// TODO: this is currently inconsistent with the Bass runtimme which allows
	// ../ to escape the context dir.
	//
	// it would be nice to ALWAYS restrict to the context dir. the runtime is
	// given an exception for now, but it's worth reconsidering.
	realPath, err := path.checkEscape()
	if err != nil {
		return nil, err
	}

	return os.Open(realPath)
}

func (value HostPath) fpath() string {
	return filepath.Join(value.ContextDir, value.Path.FilesystemPath().FromSlash())
}
