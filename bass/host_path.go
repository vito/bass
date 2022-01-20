package bass

import (
	"context"
	"fmt"
	"path/filepath"
)

// HostPath is a Path representing an absolute path on the host machine's
// filesystem.
type HostPath struct {
	ContextDir string        `json:"context"`
	Path       FileOrDirPath `json:"path"`
}

func (hp HostPath) FromSlash() string {
	return filepath.Join(hp.ContextDir, hp.Path.FilesystemPath().FromSlash())
}

var _ Value = HostPath{}

func NewHostPath(contextDir string) HostPath {
	return HostPath{
		ContextDir: contextDir,
		Path:       ParseFileOrDirPath("."),
	}
}

func (value HostPath) String() string {
	return fmt.Sprintf("<host: %s>", value.fpath())
}

func (value HostPath) Equal(other Value) bool {
	var o HostPath
	return other.Decode(&o) == nil &&
		value.ContextDir == o.ContextDir &&
		value.Path == o.Path
}

func (value HostPath) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *HostPath:
		*x = value
		return nil
	case *Path:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Applicative:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	case Decodable:
		return x.FromValue(value)
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value *HostPath) FromValue(val Value) error {
	var obj *Scope
	if err := val.Decode(&obj); err != nil {
		return fmt.Errorf("%T.FromValue: %w", value, err)
	}

	return decodeStruct(obj, value)
}

// Eval returns the value.
func (value HostPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = ThunkPath{}

func (app HostPath) Unwrap() Combiner {
	if app.Path.File != nil {
		return ThunkOperative{
			Cmd: ThunkCmd{
				Host: &app,
			},
		}
	} else {
		return ExtendOperative{app}
	}
}

var _ Combiner = HostPath{}

func (combiner HostPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(combiner.Unwrap()).Call(ctx, val, scope, cont)
}

var _ Path = HostPath{}

func (path HostPath) Name() string {
	return filepath.Base(path.fpath())
}

func (path HostPath) Extend(ext Path) (Path, error) {
	extended := path

	var err error
	extended.Path, err = path.Path.Extend(ext)
	if err != nil {
		return nil, err
	}

	return extended, nil
}

func (value HostPath) fpath() string {
	return filepath.Join(value.ContextDir, value.Path.FilesystemPath().FromSlash())
}
