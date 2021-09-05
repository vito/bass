package internal

import (
	"bytes"
	"context"

	"github.com/vito/bass"
	"gopkg.in/yaml.v3"
)

var Scope *bass.Scope = bass.NewEmptyScope()

func init() {
	Scope.Defn("yaml-decode", "[workload-path]", func(ctx context.Context, path bass.WorkloadPath) (bass.Value, error) {
		pool, err := bass.RuntimeFromContext(ctx)
		if err != nil {
			return nil, err
		}

		buf := new(bytes.Buffer)
		err = pool.Export(ctx, buf, path.Workload, path.Path.FilesystemPath())
		if err != nil {
			return nil, err
		}

		var v interface{}
		err = yaml.NewDecoder(buf).Decode(&v)
		if err != nil {
			return nil, err
		}

		return bass.ValueOf(v)
	})
}
