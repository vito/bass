package bass

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/vito/bass/pkg/proto"
	"github.com/zeebo/xxh3"
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

type Filesystem interface {
	FS(root string) fs.FS
	Write(path string, r io.Reader) error
}

// The filesystem for host paths. Re-assign it to DiscardFilesystem to disable
// writing to the host.
var FS Filesystem = HostFilesystem{}

type HostFilesystem struct{}

// AtomicSuffix is appended to the Write destination to form a path used for
// atomic writes.
const AtomicSuffix = ".new"

func (HostFilesystem) FS(root string) fs.FS { return os.DirFS(root) }

func (HostFilesystem) Write(path string, r io.Reader) error {
	atomic := path + AtomicSuffix

	f, err := os.Create(atomic)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	return os.Rename(atomic, path)
}

type DiscardFilesystem struct{}

func (DiscardFilesystem) FS(root string) fs.FS { return os.DirFS(root) }

func (DiscardFilesystem) Write(path string, r io.Reader) error { return nil }

func (value HostPath) String() string {
	return fmt.Sprintf("<host: %s>", value.fpath())
}

// Hash returns a non-cryptographic hash of the host path's context dir.
func (value HostPath) Hash() string {
	return b32(xxh3.HashString(value.ContextDir))
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
	case *Writable:
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

func (path *HostPath) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.HostPath)
	if !ok {
		return fmt.Errorf("unmarshal proto: %w", DecodeError{msg, path})
	}

	path.ContextDir = p.Context

	return path.Path.UnmarshalProto(p.Path)
}

// Eval returns the value.
func (value HostPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = ThunkPath{}

func (app HostPath) Unwrap() Combiner {
	if app.Path.File != nil {
		return ThunkOperative{
			Cmd: app,
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
	return path.Path.FilesystemPath().Name()
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

var _ Readable = HostPath{}

func (path HostPath) CachePath(_ctx context.Context, _dest string) (string, error) {
	abs, _, err := path.checkEscape()
	if err != nil {
		return "", err
	}

	return abs, nil
}

func (path HostPath) Open(context.Context) (io.ReadCloser, error) {
	// TODO: this is currently inconsistent with the Bass runtimme which allows
	// ../ to escape the context dir.
	//
	// it would be nice to ALWAYS restrict to the context dir. the runtime is
	// given an exception for now, but it's worth reconsidering.
	_, rel, err := path.checkEscape()
	if err != nil {
		return nil, err
	}

	return FS.FS(path.ContextDir).Open(rel)
}

func (path HostPath) Write(ctx context.Context, src io.Reader) error {
	// TODO: this is currently inconsistent with the Bass runtimme which allows
	// ../ to escape the context dir.
	//
	// it would be nice to ALWAYS restrict to the context dir. the runtime is
	// given an exception for now, but it's worth reconsidering.
	abs, _, err := path.checkEscape()
	if err != nil {
		return err
	}

	return FS.Write(abs, src)
}

func (value HostPath) Dir() HostPath {
	cp := value

	if value.Path.Dir != nil {
		parent := value.Path.Dir.Dir()
		cp.Path = FileOrDirPath{Dir: &parent}
	} else {
		parent := value.Path.File.Dir()
		cp.Path = FileOrDirPath{Dir: &parent}
	}

	return cp
}

func (value HostPath) fpath() string {
	return filepath.Join(value.ContextDir, value.Path.FilesystemPath().FromSlash())
}

func (path HostPath) checkEscape() (string, string, error) {
	r := filepath.Clean(path.fpath())
	c := filepath.Clean(path.ContextDir)

	rel, err := filepath.Rel(c, r)
	if err != nil {
		return "", "", err
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", HostPathEscapeError{
			ContextDir: c,
			Attempted:  r,
		}
	}

	return r, rel, nil
}
