package cli

import (
	"bytes"
	"path"
	"path/filepath"
	"strings"

	"github.com/vito/bass/pkg/bass"
)

func InputsSource(inputs []string) (*bass.Source, error) {
	bindings := bass.Bindings{}
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

		bindings[bass.Symbol(name)] = val
	}

	inputJSON, err := bass.MarshalJSON(bindings.Scope())
	if err != nil {
		return nil, err
	}

	return bass.NewSource(bass.NewJSONSource("inputs", bytes.NewBuffer(inputJSON))), nil
}
