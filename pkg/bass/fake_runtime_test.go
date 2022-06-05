package bass_test

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/fs"
	"testing/fstest"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/proto"
	gproto "google.golang.org/protobuf/proto"
)

type FakeRuntime struct {
	ExportPaths []ExportPath
}

type ExportPath struct {
	ThunkPath *proto.ThunkPath
	FS        fstest.MapFS
}

func (fake *FakeRuntime) Resolve(context.Context, *proto.ThunkImageRef) (*proto.ThunkImageRef, error) {
	return nil, fmt.Errorf("Resolve unimplemented")
}

func (fake *FakeRuntime) Run(context.Context, io.Writer, *proto.Thunk) error {
	return fmt.Errorf("Run unimplemented")
}

func (fake *FakeRuntime) Export(context.Context, io.Writer, *proto.Thunk) error {
	return fmt.Errorf("Export unimplemented")
}

func (fake *FakeRuntime) SetExportPath(path *proto.ThunkPath, fs fstest.MapFS) {
	fake.ExportPaths = append([]ExportPath{{path, fs}}, fake.ExportPaths...)
}

func (fake *FakeRuntime) ExportPath(ctx context.Context, w io.Writer, path *proto.ThunkPath) error {
	for _, setup := range fake.ExportPaths {
		if gproto.Equal(setup.ThunkPath, path) {
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

	return fmt.Errorf("thunk path not faked out: %s", path.Repr())
}

func (fake *FakeRuntime) Prune(context.Context, bass.PruneOpts) error {
	return fmt.Errorf("Prune unimplemented")
}

func (fake *FakeRuntime) Close() error {
	return nil
}
