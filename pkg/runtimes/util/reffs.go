package util

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/moby/buildkit/client/llb"
	gwclient "github.com/moby/buildkit/frontend/gateway/client"
	fstypes "github.com/tonistiigi/fsutil/types"
)

type refFS struct {
	ctx context.Context
	ref gwclient.Reference
}

func NewRefFS(ctx context.Context, ref gwclient.Reference) fs.FS {
	return &refFS{ctx: ctx, ref: ref}
}

func OpenRefFS(ctx context.Context, gw gwclient.Client, st llb.State, opts ...llb.ConstraintsOpt) (fs.FS, error) {
	def, err := st.Marshal(ctx, opts...)
	if err != nil {
		return nil, err
	}

	res, err := gw.Solve(ctx, gwclient.SolveRequest{
		Definition: def.ToPB(),
	})
	if err != nil {
		return nil, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return nil, err
	}

	if ref == nil {
		return nil, fmt.Errorf("no ref returned")
	}

	return &refFS{ctx: ctx, ref: ref}, nil
}

func (fs *refFS) Open(name string) (fs.File, error) {
	stat, err := fs.ref.StatFile(fs.ctx, gwclient.StatRequest{Path: name})
	if err != nil {
		return nil, err
	}

	return &refFile{ctx: fs.ctx, ref: fs.ref, stat: stat, name: name}, nil
}

type refFile struct {
	ctx    context.Context
	ref    gwclient.Reference
	name   string
	stat   *fstypes.Stat
	offset int64
}

func (f *refFile) Stat() (fs.FileInfo, error) {
	return &refFileInfo{stat: f.stat}, nil
}

func (f *refFile) Read(p []byte) (int, error) {
	if f.offset >= f.stat.Size_ {
		return 0, io.EOF
	}

	content, err := f.ref.ReadFile(f.ctx, gwclient.ReadRequest{
		Filename: f.name,
		Range: &gwclient.FileRange{
			Offset: int(f.offset),
			Length: len(p),
		},
	})
	if err != nil {
		return 0, err
	}
	n := copy(p, content)
	f.offset += int64(n)
	return n, nil
}

func (fi *refFile) Close() error {
	return nil
}

type refFileInfo struct {
	stat *fstypes.Stat
}

func (fi *refFileInfo) Name() string {
	return fi.stat.Path
}

func (fi *refFileInfo) Size() int64 {
	return int64(fi.stat.Size_) // NB: *NOT* Size()!
}

func (fi *refFileInfo) Mode() fs.FileMode {
	return fs.FileMode(fi.stat.Mode)
}

func (fi *refFileInfo) ModTime() time.Time {
	return time.Unix(fi.stat.ModTime/int64(time.Second), fi.stat.ModTime%int64(time.Second))
}

func (fi *refFileInfo) IsDir() bool {
	return fi.stat.IsDir()
}

func (fi *refFileInfo) Sys() interface{} {
	return nil
}
