package bass

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/vito/bass/pkg/proto"
	"google.golang.org/protobuf/encoding/protojson"
)

// FilesystemPath is a Path representing a file or directory in a filesystem.
type FilesystemPath interface {
	Path

	// Slash returns the path representation with forward slash path separators.
	Slash() string

	// FromSlash uses filepath.FromSlash to convert the path to host machine's
	// path separators.
	FromSlash() string

	// IsDir returns true if the path refers to a directory.
	IsDir() bool

	// Dir returns the parent directory of the path, or the same directory if
	// there is no parent.
	Dir() DirPath
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

func IsPathLike(arg string) bool {
	return strings.HasPrefix(arg, "./") ||
		strings.HasPrefix(arg, "/") ||
		strings.HasPrefix(arg, "../")
}

// FileOrDirPath is an enum type that accepts a FilePath or a DirPath.
type FileOrDirPath struct {
	File *FilePath
	Dir  *DirPath
}

func NewFileOrDirPath(path FilesystemPath) FileOrDirPath {
	var fp FilePath
	if err := path.Decode(&fp); err == nil {
		return FileOrDirPath{
			File: &fp,
		}
	}

	var dp DirPath
	if err := path.Decode(&dp); err == nil {
		return FileOrDirPath{
			Dir: &dp,
		}
	}

	panic(fmt.Sprintf("absurd: non-File or Dir FilesystemPath: %T", path))
}

// Slash calls Slash on whichever value is present.
func (path FileOrDirPath) Slash() string {
	return path.FilesystemPath().Slash()
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

func (path *FileOrDirPath) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.FilesystemPath)
	if !ok {
		return DecodeError{msg, path}
	}

	if p.GetDir() != nil {
		path.Dir = &DirPath{}
		return path.Dir.UnmarshalProto(p.GetDir())
	} else {
		path.File = &FilePath{}
		return path.File.UnmarshalProto(p.GetFile())
	}
}

func (value FileOrDirPath) MarshalJSON() ([]byte, error) {
	msg, err := value.MarshalProto()
	if err != nil {
		return nil, err
	}

	return protojson.Marshal(msg)
}

func (value *FileOrDirPath) UnmarshalJSON(b []byte) error {
	msg := &proto.FilesystemPath{}
	err := protojson.Unmarshal(b, msg)
	if err != nil {
		return err
	}

	return value.UnmarshalProto(msg)
}
