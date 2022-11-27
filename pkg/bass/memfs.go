package bass

import (
	"fmt"
	"path"

	"github.com/psanford/memfs"
)

// NewInMemoryFSDir is exposed as (mkfs) - it takes alternating file paths and
// content and constructs an in-memory filesystem path.
func NewInMemoryFSDir(fileContentPairs ...Value) (*FSPath, error) {
	if len(fileContentPairs)%2 != 0 {
		return nil, fmt.Errorf("mkfs: %w: odd pairing", ErrBadSyntax)
	}

	mfs := memfs.New()
	var file FilePath
	for i, val := range fileContentPairs {
		if i%2 == 0 {
			// path
			if err := val.Decode(&file); err != nil {
				return nil, fmt.Errorf("arg %d: decode: %w", i+1, err)
			}
		} else {
			var content string
			if err := val.Decode(&content); err != nil {
				return nil, fmt.Errorf("arg %d: decode: %w", i+1, err)
			}

			if err := mfs.MkdirAll(path.Dir(file.Slash()), 0755); err != nil {
				return nil, fmt.Errorf("arg %d: mkdir: %w", i+1, err)
			}

			if err := mfs.WriteFile(path.Clean(file.Slash()), []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("arg %d: write: %w", i+1, err)
			}
		}
	}

	return NewFSPath(mfs, FileOrDirPath{Dir: &DirPath{"."}}), nil
}

func NewInMemoryFile(name string, content string) *FSPath {
	mfs := memfs.New()
	_ = mfs.MkdirAll(path.Dir(name), 0755)
	_ = mfs.WriteFile(name, []byte(content), 0644)

	return NewFSPath(mfs, ParseFileOrDirPath(name))
}
