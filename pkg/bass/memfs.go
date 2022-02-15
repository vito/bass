package bass

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"time"
)

// NewInMemoryFSDir is exposed as (mkfs) - it takes alternating file paths and
// content and constructs an in-memory filesystem path.
func NewInMemoryFSDir(fileContentPairs ...Value) (FSPath, error) {
	if len(fileContentPairs)%2 != 0 {
		return FSPath{}, fmt.Errorf("mkfs: %w: odd pairing", ErrBadSyntax)
	}

	memfs := InMemoryFS{}
	var file FilePath
	for i, val := range fileContentPairs {
		if i%2 == 0 {
			// path
			if err := val.Decode(&file); err != nil {
				return FSPath{}, fmt.Errorf("arg %d: %w", i+1, err)
			}
		} else {
			var content string
			if err := val.Decode(&content); err != nil {
				return FSPath{}, fmt.Errorf("arg %d: %w", i+1, err)
			}

			memfs[path.Clean(file.String())] = content
		}
	}

	id, err := memfs.SHA256()
	if err != nil {
		return FSPath{}, err
	}

	return NewFSPath(id, memfs, FileOrDirPath{Dir: &DirPath{"."}}), nil
}

// InMemoryFS is a stupid simple filesystem representation, not even concerning
// itself with pesky permissions and timestamps.
//
// It maps cleaned file paths to their content. It does not contain empty
// directories, but its file paths may be nested.
type InMemoryFS map[string]string

// SHA256 returns a checksum of the filesystem.
func (inmem InMemoryFS) SHA256() (string, error) {
	idSum := sha256.New()

	sorted := []string{}
	for f := range inmem {
		sorted = append(sorted, f)
	}
	sort.Strings(sorted)

	for _, file := range sorted {
		if _, err := idSum.Write([]byte(file)); err != nil {
			return "", err
		}

		content := inmem[file]
		if _, err := idSum.Write([]byte(content)); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", idSum.Sum(nil)), nil
}

// Opens returns a file for reading the given file's content or errors if the
// file does not exist.
//
// The returned file always has 0644 permissions and a zero (Unix epoch) mtime.
func (inmem InMemoryFS) Open(name string) (fs.File, error) {
	content, found := inmem[name]
	if !found {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	return &inMemoryFile{
		name: name,

		Buffer: bytes.NewBufferString(content),
	}, nil
}

type inMemoryFile struct {
	name string
	size int64

	*bytes.Buffer
}

func (file *inMemoryFile) Stat() (fs.FileInfo, error) {
	return &inMemoryInfo{
		name: path.Base(file.name),
		size: file.size,
		mode: 0644,
	}, nil
}

func (file *inMemoryFile) Close() error {
	return nil
}

type inMemoryInfo struct {
	name string
	size int64
	mode fs.FileMode
}

func (info *inMemoryInfo) Name() string {
	return info.name
}

func (info *inMemoryInfo) Size() int64 {
	return info.size
}

func (info *inMemoryInfo) ModTime() time.Time {
	return time.Time{}
}

func (info *inMemoryInfo) Mode() fs.FileMode {
	return info.mode
}

func (info *inMemoryInfo) IsDir() bool {
	return false
}

func (info *inMemoryInfo) Sys() interface{} {
	return nil
}
