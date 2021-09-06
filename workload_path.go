package bass

import (
	"context"
	"fmt"
)

// A path created by a workload.
type WorkloadPath struct {
	Workload Workload      `json:"workload"`
	Path     FileOrDirPath `json:"path"`
}

var _ Value = WorkloadPath{}

func (value WorkloadPath) String() string {
	return fmt.Sprintf("%s/%s", value.Workload, value.Path)
}

func (value WorkloadPath) Equal(other Value) bool {
	var o WorkloadPath
	return other.Decode(&o) == nil &&
		value.Path.ToValue().Equal(o.Path.ToValue())
}

func (value WorkloadPath) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *WorkloadPath:
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

func (value *WorkloadPath) FromValue(val Value) error {
	var obj *Scope
	if err := val.Decode(&obj); err != nil {
		return fmt.Errorf("%T.FromValue: %w", value, err)
	}

	return decodeStruct(obj, value)
}

// Eval returns the value.
func (value WorkloadPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = WorkloadPath{}

func (app WorkloadPath) Unwrap() Combiner {
	return PathOperative{app}
}

var _ Combiner = WorkloadPath{}

func (combiner WorkloadPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(PathOperative{combiner}).Call(ctx, val, scope, cont)
}

var _ Path = WorkloadPath{}

func (path WorkloadPath) Extend(ext Path) (Path, error) {
	extended := path

	var err error
	extended.Path, err = path.Path.Extend(ext)
	if err != nil {
		return nil, err
	}

	return extended, nil
}
