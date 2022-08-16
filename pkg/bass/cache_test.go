package bass_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/is"
	"golang.org/x/sync/errgroup"
)

type fakeReadable struct {
	bass.Value
	OpenFunc func(ctx context.Context) (io.ReadCloser, error)
}

var _ bass.Readable = &fakeReadable{}

func (r *fakeReadable) CachePath(ctx context.Context, dest string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (r *fakeReadable) Open(ctx context.Context) (io.ReadCloser, error) {
	return r.OpenFunc(ctx)
}

func TestCache(t *testing.T) {
	is := is.New(t)

	content := strings.Repeat("hello", 1024)

	ctx := context.Background()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cache")
	readable := &fakeReadable{
		Value: bass.Null{},
		OpenFunc: func(ctx context.Context) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewBufferString(content)), nil
		},
	}

	eg := new(errgroup.Group)

	for i := 0; i < 1000; i++ {
		i := i

		eg.Go(func() error {
			cached, err := bass.Cache(ctx, path, readable)
			if err != nil {
				return err
			}

			bytes, err := os.ReadFile(cached)
			if err != nil {
				return err
			}

			if content != string(bytes) {
				return fmt.Errorf("%q != %q", string(bytes), content)
			}

			t.Logf("verified %d", i)

			return nil
		})
	}

	is.NoErr(eg.Wait())
}
