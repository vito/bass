package bass

import "context"

type Error struct {
	Err error
}

var _ Value = Error{}

// Eval calls the continuation with the error.
func (value Error) Equal(o Value) bool {
	var other Error
	return o.Decode(&other) == nil && value.Err == other.Err
}

// Eval calls the continuation with the error.
func (value Error) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

// String returns the error message.
func (value Error) String() string {
	return "<error: " + value.Err.Error() + ">"
}

// String returns the error message.
func (value Error) Decode(dest any) error {
	switch x := dest.(type) {
	case *error:
		*x = value.Err
		return nil
	case *Error:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

var _ Combiner = Error{}

func (value Error) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(nil, value.Err)
}
