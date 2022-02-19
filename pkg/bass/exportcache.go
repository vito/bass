package bass

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

// CacheThunkPath exports a thunk path to a local cache under $XDG_CACHE_HOME.
//
// If the path is a file, a path to the cached file will be returned.
//
// If the path is a directory, a path to a cached directory containing its
// immediate files will be returned. Note that sub-directories are not
// recursively exported.
//
// A cached directory will be marked as cached by placing a .cached file in it
// to distinguish from a directory created in order to export a child path.
//
// It does not preserve things like file permissions and timestamps. It is only
// for accessing the content of files.
func CacheThunkPath(ctx context.Context, tp ThunkPath) (string, error) {
	sha, err := tp.SHA256() // sha of the path, not just its thunk
	if err != nil {
		return "", err
	}

	cachePath, err := xdg.CacheFile(filepath.Join("bass", "export", sha))
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	pool, err := RuntimePoolFromContext(ctx)
	if err != nil {
		return "", err
	}

	runt, err := pool.Select(tp.Thunk.Platform())
	if err != nil {
		return "", err
	}

	src := new(bytes.Buffer)
	err = runt.ExportPath(ctx, src, tp)
	if err != nil {
		return "", fmt.Errorf("export thunk path: %w", err)
	}

	tr := tar.NewReader(src)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return "", fmt.Errorf("export %s: %w", tp, err)
		}

		if tp.Path.FilesystemPath().IsDir() && (strings.Contains(hdr.Name, "/") || hdr.Typeflag == tar.TypeDir) {
			// NB: do not recurse into sub-directories; we only cache directory
			// exports when we're looking for a particular file, so it's better to be
			// explicit rather than needlessly hoist around things like .git/...
			continue
		}

		// TODO: handle symlinks? might be necessary for bass.lock

		var dest string
		if tp.Path.FilesystemPath().IsDir() {
			dest = filepath.Join(cachePath, hdr.Name)
		} else {
			dest = cachePath
		}

		err = os.MkdirAll(filepath.Dir(dest), 0755)
		if err != nil {
			return "", err
		}

		cacheFile, err := os.Create(dest)
		if err != nil {
			return "", fmt.Errorf("create cache: %w", err)
		}

		_, err = io.Copy(cacheFile, tr)
		if err != nil {
			return "", err
		}

		err = cacheFile.Close()
		if err != nil {
			return "", err
		}
	}

	return cachePath, nil
}
