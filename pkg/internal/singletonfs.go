package internal

import (
	"fmt"
	"io"
	"io/fs"
	"path"
)

type SingletonFS struct {
	Name string
	Info fs.FileInfo
	io.ReadCloser
}

func (fs SingletonFS) Open(name string) (fs.File, error) {
	if path.Clean(name) != path.Clean(fs.Name) {
		return nil, fmt.Errorf("name mismatch: %s != %s", name, fs.Name)
	}

	return fs, nil
}

func (fs SingletonFS) Stat() (fs.FileInfo, error) {
	if fs.Info != nil {
		return fs.Info, nil
	}

	return nil, fmt.Errorf("%s: no file info", fs.Name)
}
