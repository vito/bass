package bass

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// FilesystemPath is a Path representing a file or directory in a filesystem.
type FilesystemPath interface {
	Path

	// FromSlash uses filepath.FromSlash to convert the path to host machine's
	// path separators.
	FromSlash() string

	// IsDir() returns true if the path refers to a directory.
	IsDir() bool
}

// ParseFileOrDirPath parses arg as a path using the host machine's separator
// convention.
//
// If the path is '.' or has a trailing slash, a DirPath is returned.
//
// Otherwise, a FilePath is returned.
func ParseFileOrDirPath(arg string) FileOrDirPath {
	p := filepath.ToSlash(arg)

	isDir := arg == "." || strings.HasSuffix(p, "/")

	var fod FileOrDirPath
	if isDir {
		fod.Dir = &DirPath{
			// trim suffix left behind from Clean returning "/"
			Path: strings.TrimSuffix(path.Clean(p), "/"),
		}
	} else {
		fod.File = &FilePath{
			Path: path.Clean(p),
		}
	}

	return fod
}

// FileOrDirPath is an enum type that accepts a FilePath or a DirPath.
type FileOrDirPath struct {
	File *FilePath
	Dir  *DirPath
}

// String calls String on whichever value is present.
func (path FileOrDirPath) String() string {
	return path.ToValue().String()
}

// FilesystemPath returns the value present.
func (path FileOrDirPath) FilesystemPath() FilesystemPath {
	return path.ToValue().(FilesystemPath)
}

// Extend extends the value with the given path and returns it wrapped in
// another FileOrDirPath.
func (path FileOrDirPath) Extend(ext Path) (FileOrDirPath, error) {
	extended := &FileOrDirPath{}

	ext, err := path.ToValue().(Path).Extend(ext)
	if err != nil {
		return FileOrDirPath{}, err
	}

	err = extended.FromValue(ext)
	if err != nil {
		return FileOrDirPath{}, err
	}

	return *extended, nil
}

var _ Decodable = &FileOrDirPath{}
var _ Encodable = FileOrDirPath{}

// ToValue returns the value present.
func (path FileOrDirPath) ToValue() Value {
	if path.File != nil {
		return *path.File
	} else {
		return *path.Dir
	}
}

// UnmarshalJSON unmarshals a FilePath or DirPath from JSON.
func (path *FileOrDirPath) UnmarshalJSON(payload []byte) error {
	var obj *Scope
	err := UnmarshalJSON(payload, &obj)
	if err != nil {
		return err
	}

	return path.FromValue(obj)
}

func (path FileOrDirPath) MarshalJSON() ([]byte, error) {
	return MarshalJSON(path.ToValue())
}

// FromValue decodes val into a FilePath or a DirPath, setting whichever worked
// as the internal value.
func (path *FileOrDirPath) FromValue(val Value) error {
	var file FilePath
	if err := val.Decode(&file); err == nil {
		path.File = &file
		return nil
	}

	var dir DirPath
	if err := val.Decode(&dir); err == nil {
		path.Dir = &dir
		return nil
	}

	return DecodeError{
		Source:      val,
		Destination: path,
	}
}

// FSPath is a Path representing a file or directory relative to a filesystem.
//
// This type will typically never occur in production code. It is only used for
// embedded filesystems, i.e. in Bass's stdlib and test suites.
//
// JSON tags are specified just for keeping up appearances - this type needs to
// be marshalable just to support .SHA1, .SHA256, .Avatar, etc. on a Thunk
// that embeds it.
type FSPath struct {
	FS   fs.FS         `json:"fs"`
	Path FileOrDirPath `json:"path"`
}

func NewFSDir(fs fs.FS) FSPath {
	return FSPath{
		FS: fs,
		Path: FileOrDirPath{
			Dir: &DirPath{"."},
		},
	}
}

var _ Value = FSPath{}

func (value FSPath) String() string {
	return fmt.Sprintf("(fs)/%s", strings.TrimPrefix(value.Path.String(), "./"))
}

func (value FSPath) Equal(other Value) bool {
	var o FSPath
	return other.Decode(&o) == nil &&
		value.Path.ToValue().Equal(o.Path.ToValue())
}

func (value FSPath) Decode(dest interface{}) error {
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
	return PathOperative{app}
}

var _ Combiner = FSPath{}

func (combiner FSPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(PathOperative{combiner}).Call(ctx, val, scope, cont)
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
