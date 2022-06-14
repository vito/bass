package bass_test

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/fs"
	"testing/fstest"

	"github.com/vito/bass/pkg/bass"
)

type FakeRuntime struct {
	ExportPaths []ExportPath
}

type ExportPath struct {
	ThunkPath bass.ThunkPath
	FS        fstest.MapFS
}

func (fake *FakeRuntime) Resolve(context.Context, bass.ThunkImageRef) (bass.ThunkImageRef, error) {
	return bass.ThunkImageRef{}, fmt.Errorf("Resolve unimplemented")
}

func (fake *FakeRuntime) Run(context.Context, bass.Thunk) error {
	return fmt.Errorf("Run unimplemented")
}

func (fake *FakeRuntime) Read(context.Context, io.Writer, bass.Thunk) error {
	return fmt.Errorf("Read unimplemented")
}

func (fake *FakeRuntime) Load(context.Context, bass.Thunk) (*bass.Scope, error) {
	return nil, fmt.Errorf("Load unimplemented")
}

func (fake *FakeRuntime) Export(context.Context, io.Writer, bass.Thunk) error {
	return fmt.Errorf("Export unimplemented")
}

func (fake *FakeRuntime) SetExportPath(path bass.ThunkPath, fs fstest.MapFS) {
	fake.ExportPaths = append([]ExportPath{{path, fs}}, fake.ExportPaths...)
}

func (fake *FakeRuntime) ExportPath(ctx context.Context, w io.Writer, path bass.ThunkPath) error {
	for _, setup := range fake.ExportPaths {
		if setup.ThunkPath.Equal(path) {
			tarWriter := tar.NewWriter(w)
			defer tarWriter.Close()

			err := fs.WalkDir(setup.FS, ".", func(filePath string, dirEntry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if dirEntry.IsDir() {
					return nil
				}
				info, err := dirEntry.Info()
				if err != nil {
					return err
				}
				header, err := tar.FileInfoHeader(info, filePath)
				if err != nil {
					return err
				}
				header.Name = filePath
				if err := tarWriter.WriteHeader(header); err != nil {
					return err
				}
				file, err := setup.FS.Open(filePath)
				if err != nil {
					return err
				}
				if _, err := io.Copy(tarWriter, file); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}

			if err := tarWriter.Flush(); err != nil {
				return err
			}

			return nil
		}
	}

	return fmt.Errorf("thunk path not faked out: %s", path)
}

func (fake *FakeRuntime) Prune(context.Context, bass.PruneOpts) error {
	return fmt.Errorf("Prune unimplemented")
}

func (fake *FakeRuntime) Close() error {
	return nil
}
