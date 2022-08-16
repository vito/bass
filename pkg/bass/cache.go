package bass

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

// CacheHome is the directory where Bass stores caches.
var CacheHome string

func init() {
	CacheHome = filepath.Join(xdg.CacheHome, "bass")
}

// Cache exports a readable file path to a local path if it does not already
// exist.
//
// Does not preserve file permissions and timestamps. Only use for accessing
// the content of a Readable.
func Cache(ctx context.Context, cachePath string, rd Readable) (string, error) {
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	rc, err := rd.Open(ctx)
	if err != nil {
		return "", fmt.Errorf("cache: open readable: %w", err)
	}

	defer rc.Close()

	parent := filepath.Dir(cachePath)
	err = os.MkdirAll(parent, 0700)
	if err != nil {
		return "", fmt.Errorf("cache: mkdir cache parent: %w", err)
	}

	tmpFile, err := os.CreateTemp(parent, filepath.Base(cachePath)+".*")
	if err != nil {
		return "", fmt.Errorf("cache: create temp: %w", err)
	}

	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, rc)
	if err != nil {
		return "", fmt.Errorf("cache: write tmp: %w", err)
	}

	err = os.Rename(tmpFile.Name(), cachePath)
	if err != nil {
		return "", fmt.Errorf("cache: rename %s -> %s: %w", tmpFile.Name(), cachePath, err)
	}

	return cachePath, nil
}
