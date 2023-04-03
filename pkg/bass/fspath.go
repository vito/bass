package bass

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/psanford/memfs"
	"github.com/vito/bass/pkg/proto"
	"github.com/zeebo/xxh3"
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
	FS   fs.FS         `json:"-"`
	Path FileOrDirPath `json:"path"`
}

func NewFSDir(fs fs.FS) *FSPath {
	return NewFSPath(fs, ParseFileOrDirPath("."))
}

func NewFSPath(fs fs.FS, path FileOrDirPath) *FSPath {
	return &FSPath{
		FS:   fs,
		Path: path,
	}
}

var _ Value = (*FSPath)(nil)

func (value *FSPath) String() string {
	return fmt.Sprintf("<fs>/%s", strings.TrimPrefix(value.Path.Slash(), "./"))
}

func (value *FSPath) Equal(other Value) bool {
	var o *FSPath
	return other.Decode(&o) == nil && value == o
}

func (value *FSPath) Decode(dest any) error {
	switch x := dest.(type) {
	case **FSPath:
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
func (value *FSPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = ThunkPath{}

func (app *FSPath) Unwrap() Combiner {
	if app.Path.File != nil {
		return ThunkOperative{
			Cmd: app,
		}
	} else {
		return ExtendOperative{app}
	}
}

var _ Combiner = (*FSPath)(nil)

func (combiner *FSPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(combiner.Unwrap()).Call(ctx, val, scope, cont)
}

var _ Path = (*FSPath)(nil)

func (path *FSPath) Name() string {
	// TODO: should this special-case ./ to return the path hash?
	return path.Path.FilesystemPath().Name()
}

func (path *FSPath) Extend(ext Path) (Path, error) {
	extended := *path

	var err error
	extended.Path, err = path.Path.Extend(ext)
	if err != nil {
		return nil, err
	}

	return &extended, nil
}

var _ Readable = (*FSPath)(nil)

func (fsp *FSPath) CachePath(ctx context.Context, dest string) (string, error) {
	hash, err := fsp.Hash()
	if err != nil {
		return "", err
	}

	return Cache(ctx, filepath.Join(dest, "fs", hash, fsp.Path.FilesystemPath().FromSlash()), fsp)
}

func (fsp *FSPath) Open(ctx context.Context) (io.ReadCloser, error) {
	return fsp.FS.Open(path.Clean(fsp.Path.Slash()))
}

func (value *FSPath) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.LogicalPath)
	if !ok {
		return DecodeError{msg, value}
	}

	switch x := p.Path.(type) {
	case *proto.LogicalPath_File_:
		value.Path = FileOrDirPath{
			File: &FilePath{Path: x.File.Name},
		}
	case *proto.LogicalPath_Dir_:
		value.Path = FileOrDirPath{
			Dir: &DirPath{Path: x.Dir.Name},
		}
	default:
		return fmt.Errorf("impossible: non-file-or-dir path: %T", x)
	}

	mfs := memfs.New()
	value.FS = mfs

	return loadFS(mfs, ".", p)
}

// Hash returns a non-cryptographic hash of the filesystem.
func (value *FSPath) Hash() (string, error) {
	idSum := xxh3.New()

	err := fs.WalkDir(value.FS, path.Clean(value.Path.Slash()), func(name string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if _, err := idSum.Write([]byte(name + "\x00")); err != nil {
			return err
		}

		rc, err := value.FS.Open(name)
		if err != nil {
			return fmt.Errorf("open %s: %w", name, err)
		}

		defer rc.Close()

		_, err = io.Copy(idSum, rc)
		if err != nil {
			return fmt.Errorf("copy: %w", err)
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk: %w", err)
	}

	sum := idSum.Sum(nil)
	return base64.URLEncoding.EncodeToString(sum[:]), nil
}

func (value *FSPath) Dir() *FSPath {
	cp := *value

	if value.Path.Dir != nil {
		parent := value.Path.Dir.Dir()
		cp.Path = FileOrDirPath{Dir: &parent}
	} else {
		parent := value.Path.File.Dir()
		cp.Path = FileOrDirPath{Dir: &parent}
	}

	return &cp
}

func loadFS(mfs *memfs.FS, parent string, p *proto.LogicalPath) error {
	switch x := p.GetPath().(type) {
	case *proto.LogicalPath_File_:
		if err := mfs.MkdirAll(parent, 0755); err != nil {
			return fmt.Errorf("mkdir parent: %w", err)
		}

		fp := path.Join(parent, x.File.Name)
		if err := mfs.WriteFile(fp, []byte(x.File.Content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", fp, err)
		}

		return nil

	case *proto.LogicalPath_Dir_:
		sub := path.Join(parent, x.Dir.Name)
		for _, child := range x.Dir.Entries {
			if err := loadFS(mfs, sub, child); err != nil {
				return fmt.Errorf("%s: %w", x.Dir.Name, err)
			}
		}

		return nil

	default:
		return fmt.Errorf("impossible: non-file-or-dir path: %T", x)
	}
}
