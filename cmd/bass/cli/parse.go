package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/vito/bass"
)

func ParseArgs(argv []string) (string, []bass.Value, error) {
	file := argv[0]

	var args []bass.Value
	for _, arg := range argv[1:] {
		var val bass.Value
		if arg == "." || strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "/") {
			var err error
			val, err = parsePathArg(arg)
			if err != nil {
				return "", nil, err
			}
		} else {
			val = bass.String(arg)
		}

		args = append(args, val)
	}

	return file, args, nil
}

func parsePathArg(arg string) (bass.Path, error) {
	path, err := filepath.Abs(arg)
	if err != nil {
		return nil, err
	}

	isDir := arg == "." || strings.HasSuffix(arg, "/")
	if !isDir {
		info, err := os.Stat(path)
		if err == nil {
			isDir = info.IsDir()
		}
	}

	if isDir {
		return bass.DirectoryPath{
			Path: path,
		}, nil
	} else {
		return bass.FilePath{
			Path: path,
		}, nil
	}
}
