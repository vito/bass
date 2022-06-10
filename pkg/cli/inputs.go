package cli

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/vito/bass/pkg/bass"
)

func InputsSource(inputs []string) *bass.Source {
	scope := bass.NewEmptyScope()

	for _, input := range inputs {
		var val bass.Value

		name, arg, ok := strings.Cut(input, "=")
		if !ok {
			val = bass.Bool(true)
		} else if bass.IsPathLike(arg) {
			dir, base := path.Split(arg)
			if base == "" {
				base = "."
			}

			val = bass.NewHostPath(filepath.FromSlash(path.Clean(dir)), bass.ParseFileOrDirPath(base))
		} else {
			val = bass.String(arg)
		}

		scope.Set(bass.Symbol(name), val)
	}

	return bass.NewSource(bass.NewInMemorySource(scope))
}
