package bass

import "context"

type Ignore struct{}

var _ Value = Ignore{}

func (value Ignore) Equal(other Value) bool {
	var o Ignore
	return other.Decode(&o) == nil
}

func (value Ignore) String() string {
	return "_"
}

func (value Ignore) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Ignore:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Bindable:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value Ignore) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Bindable = Ignore{}

func (binding Ignore) Bind(*Scope, Value) error {
	return nil
}
