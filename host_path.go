package bass

import (
	"context"
)

// HostPath is a Path representing an absolute path on the host machine's
// filesystem.
type HostPath struct {
	Path FileOrDirPath `json:"host"`
}

var _ Value = HostPath{}

func NewHostPath(localPath string) HostPath {
	return HostPath{
		Path: ParseFileOrDirPath(localPath),
	}
}

func (value HostPath) String() string {
	return value.Path.String()
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
func (value HostPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = ThunkPath{}

func (app HostPath) Unwrap() Combiner {
	return PathOperative{app}
}

var _ Combiner = HostPath{}

func (combiner HostPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(PathOperative{combiner}).Call(ctx, val, scope, cont)
}

var _ Path = HostPath{}

func (path HostPath) Extend(ext Path) (Path, error) {
	extended := path

	var err error
	extended.Path, err = path.Path.Extend(ext)
	if err != nil {
		return nil, err
	}

	return extended, nil
}
