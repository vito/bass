package bass

import "context"

type Ignore struct{}

var _ Value = Ignore{}

func (Ignore) Equal(other Value) bool {
	var o Ignore
	return other.Decode(&o) == nil
}

func (Ignore) Repr() string {
	return "_"
}

func (value Ignore) Decode(dest any) error {
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

func (value Ignore) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Bindable = Ignore{}

func (Ignore) Bind(_ context.Context, _ *Scope, cont Cont, _ Value, _ ...Annotated) ReadyCont {
	return cont.Call(Ignore{}, nil)
}

func (Ignore) EachBinding(func(Symbol, Range) error) error {
	return nil
}
