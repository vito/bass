package internal

import (
	"archive/tar"
	"context"
	"io"

	"github.com/vito/bass"
	"gopkg.in/yaml.v3"
)

var Scope *bass.Scope = bass.NewEmptyScope()

func init() {
	Scope.Set("yaml-decode",
		bass.Func("yaml-decode", "[workload-path]", func(ctx context.Context, path bass.WorkloadPath) (bass.Value, error) {
			pool, err := bass.RuntimeFromContext(ctx)
			if err != nil {
				return nil, err
			}

			r, w := io.Pipe()

			go func() {
				w.CloseWithError(pool.Export(ctx, w, path.Workload, path.Path.FilesystemPath()))
			}()

			tr := tar.NewReader(r)

			_, err = tr.Next()
			if err != nil {
				return nil, err
			}

			var v interface{}
			err = yaml.NewDecoder(tr).Decode(&v)
			if err != nil {
				return nil, err
			}

			return bass.ValueOf(v)
		}))
}
