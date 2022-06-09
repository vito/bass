package bass

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// FSPath is a Path representing a file or directory relative to a filesystem.
//
// This type will typically never occur in production code. It is only used for
// embedded filesystems, i.e. in Bass's stdlib and test suites.
//
// JSON tags are specified just for keeping up appearances - this type needs to
// be marshalable just to support .SHA256, .Name, .Avatar, etc. on a Thunk that
// embeds it.
type FSPath struct {
	ID   string        `json:"fs"`
	FS   fs.FS         `json:"-"`
	Path FileOrDirPath `json:"path"`
}

func NewFSDir(id string, fs fs.FS) FSPath {
	return NewFSPath(id, fs, ParseFileOrDirPath("."))
}

func NewFSPath(id string, fs fs.FS, path FileOrDirPath) FSPath {
	return FSPath{
		ID:   id,
		FS:   fs,
		Path: path,
	}
}

var _ Value = FSPath{}

func (value FSPath) String() string {
	return fmt.Sprintf("<fs: %s>/%s", value.ID, strings.TrimPrefix(value.Path.Slash(), "./"))
}

func (value FSPath) Equal(other Value) bool {
	var o FSPath
	return other.Decode(&o) == nil &&
		value.ID == o.ID &&
		value.Path.ToValue().Equal(o.Path.ToValue())
}

func (value FSPath) Decode(dest any) error {
	switch x := dest.(type) {
	case *FSPath:
		*x = value
		return nil
	case *Path:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Readable:
		*x = value
		return nil
	case *Applicative:
		*x = value
		return nil
	case *Combiner:
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

// Eval returns the value.
func (value FSPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = ThunkPath{}

func (app FSPath) Unwrap() Combiner {
	if app.Path.File != nil {
		return ThunkOperative{
			Cmd: ThunkCmd{
				FS: &app,
			},
		}
	} else {
		return ExtendOperative{app}
	}
}

var _ Combiner = FSPath{}

func (combiner FSPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(combiner.Unwrap()).Call(ctx, val, scope, cont)
}

var _ Path = FSPath{}

func (path FSPath) Name() string {
	return path.Path.FilesystemPath().Name()
}

func (path FSPath) Extend(ext Path) (Path, error) {
	extended := path

	var err error
	extended.Path, err = path.Path.Extend(ext)
	if err != nil {
		return nil, err
	}

	return extended, nil
}

var _ Readable = FSPath{}

func (fsp FSPath) CachePath(ctx context.Context, dest string) (string, error) {
	return Cache(ctx, filepath.Join(dest, "fs", fsp.ID, fsp.Path.FilesystemPath().FromSlash()), fsp)
}

func (fsp FSPath) Open(ctx context.Context) (io.ReadCloser, error) {
	return fsp.FS.Open(path.Clean(fsp.Path.Slash()))
}
