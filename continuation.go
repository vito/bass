package bass

import "fmt"

type Cont interface {
	Call(Value, error) ReadyCont
}

type ReadyCont interface {
	Go() (Value, ReadyCont, error)
}

type Continuation struct {
	Continue func(Value) ReadyCont
}

func Continue(cont func(Value) ReadyCont) Cont {
	return &Continuation{
		Continue: cont,
	}
}

type IdentityCont struct{}

func (cont IdentityCont) Call(val Value, err error) ReadyCont {
	return IdentityReadyCont{
		Value: val,
		Err:   err,
	}
}

type IdentityReadyCont struct {
	Value Value
	Err   error
}

func (rdy IdentityReadyCont) Go() (Value, ReadyCont, error) {
	return rdy.Value, nil, rdy.Err
}

var Identity = IdentityCont{}

func (value *Continuation) String() string {
	return "<continuation>"
}

func (value *Continuation) Eval(env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func (value *Continuation) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case **Continuation:
		*x = value
		return nil
	case *Cont:
		*x = value
		return nil
	default:
		return DecodeError{
			Destination: dest,
			Source:      value,
		}
	}
}

func (sink *Continuation) Equal(other Value) bool {
	var o *Continuation
	return other.Decode(&o) == nil && sink == o
}

func (cont *Continuation) Call(res Value, err error) ReadyCont {
	return &ReadyContinuation{
		Continuation: cont,
		Result:       res,
		Err:          err,
	}
}

type ReadyContinuation struct {
	*Continuation

	Result Value
	Err    error
}

func (cont *ReadyContinuation) String() string {
	return fmt.Sprintf("<continue: %s>", cont.Result)
}

func (cont *ReadyContinuation) Go() (Value, ReadyCont, error) {
	if cont.Err != nil {
		return nil, nil, cont.Err
	}

	return nil, cont.Continuation.Continue(cont.Result), nil
}

func (value *ReadyContinuation) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *ReadyCont:
		*x = value
		return nil
	default:
		return DecodeError{
			Destination: dest,
			Source:      value,
		}
	}
}
