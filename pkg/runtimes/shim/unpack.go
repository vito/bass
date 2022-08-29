package main

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/oci/cas"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
)

func unpack(args []string) error {
	ctx := context.Background()

	if len(args) != 3 {
		return fmt.Errorf("usage: unpack <image.tar> <tag> <dest/>")
	}

	archiveSrc := args[0]
	fromName := args[1]
	rootfsPath := args[2]

	layout, err := openTar(archiveSrc)
	if err != nil {
		return fmt.Errorf("create layout: %w", err)
	}

	defer layout.Close()

	ext := casext.NewEngine(layout)

	mspec, err := loadManifest(ctx, ext, fromName)
	if err != nil {
		return err
	}

	err = layer.UnpackRootfs(context.Background(), ext, rootfsPath, mspec, &layer.UnpackOptions{})
	if err != nil {
		return fmt.Errorf("unpack rootfs: %w", err)
	}

	return nil
}

func openTar(tarPath string) (cas.Engine, error) {
	archive, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}

	return &tarEngine{archive}, nil
}

// tarEngine implements a read-only cas.Engine backed by a .tar archive.
type tarEngine struct {
	archive *os.File
}

func (engine *tarEngine) PutBlob(ctx context.Context, reader io.Reader) (digest.Digest, int64, error) {
	return "", 0, fmt.Errorf("PutBlob: %w", cas.ErrNotImplemented)
}

func (engine *tarEngine) GetBlob(ctx context.Context, dig digest.Digest) (io.ReadCloser, error) {
	r, err := engine.open(path.Join("blobs", dig.Algorithm().String(), dig.Encoded()))
	if err != nil {
		return nil, err
	}

	return io.NopCloser(r), nil
}

func (engine *tarEngine) StatBlob(ctx context.Context, dig digest.Digest) (bool, error) {
	_, err := engine.open(path.Join("blobs", dig.Algorithm().String(), dig.Encoded()))
	if err != nil {
		if errors.Is(err, cas.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (engine *tarEngine) PutIndex(ctx context.Context, index ispec.Index) error {
	return fmt.Errorf("PutIndex: %w", cas.ErrNotImplemented)
}

func (engine *tarEngine) GetIndex(ctx context.Context) (ispec.Index, error) {
	var idx ispec.Index
	r, err := engine.open("index.json")
	if err != nil {
		return ispec.Index{}, err
	}

	err = json.NewDecoder(r).Decode(&idx)
	if err != nil {
		return ispec.Index{}, err
	}

	return idx, nil
}

func (engine *tarEngine) open(p string) (io.Reader, error) {
	_, err := engine.archive.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	tr := tar.NewReader(engine.archive)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("open %s: %w", p, cas.ErrNotExist)
			}

			return nil, err
		}

		if path.Clean(hdr.Name) == p {
			return tr, nil
		}
	}
}

func (engine *tarEngine) DeleteBlob(ctx context.Context, digest digest.Digest) (err error) {
	return fmt.Errorf("DeleteBlob: %w", cas.ErrNotImplemented)
}

func (engine *tarEngine) ListBlobs(ctx context.Context) ([]digest.Digest, error) {
	_, err := engine.archive.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	tr := tar.NewReader(engine.archive)

	var digs []digest.Digest
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("next: %w", err)
		}

		if strings.HasPrefix(path.Clean(hdr.Name), "blobs/") {
			dir, encoded := path.Split(hdr.Name)
			_, alg := path.Split(dir)
			digs = append(digs, digest.NewDigestFromEncoded(digest.Algorithm(alg), encoded))
		}
	}

	return digs, nil
}

func (engine *tarEngine) Clean(ctx context.Context) error { return nil }

func (engine *tarEngine) Close() error {
	return engine.archive.Close()
}
