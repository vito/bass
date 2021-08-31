package bass

import (
	"context"
	"path/filepath"
)

// HostPath is a Path representing an absolute path on the host machine's
// filesystem.
type HostPath struct {
	Path string `json:"host"`
}

var _ Value = HostPath{}

func (value HostPath) String() string {
	return value.Path
}

func (value HostPath) Equal(other Value) bool {
	var o HostPath
	return other.Decode(&o) == nil && value.Path == o.Path
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

// Eval returns the value.
func (value HostPath) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = WorkloadPath{}

func (app HostPath) Unwrap() Combiner {
	return PathOperative{app}
}

var _ Combiner = HostPath{}

func (combiner HostPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(PathOperative{combiner}).Call(ctx, val, scope, cont)
}

var _ Path = HostPath{}

func (path HostPath) Extend(ext Path) (Path, error) {
	path.Path = filepath.Join(path.Path, filepath.FromSlash(ext.String()))
	return path, nil
}
