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

	err = os.MkdirAll(filepath.Dir(cachePath), 0700)
	if err != nil {
		return "", fmt.Errorf("cache: mkdir parent: %w", err)
	}

	cacheFile, err := os.Create(cachePath)
	if err != nil {
		return "", fmt.Errorf("create cache: %w", err)
	}

	defer cacheFile.Close()

	_, err = io.Copy(cacheFile, rc)
	if err != nil {
		return "", err
	}

	return cachePath, nil
}
