package bass

import (
	"context"
	"fmt"
)

type Cont interface {
	Value

	Call(Value, error) ReadyCont
	Traced(*Trace) Cont
}

type ReadyCont interface {
	Value

	Go() (Value, error)
}

type Continuation struct {
	Continue    func(Value) Value
	Trace       *Trace
	TracedDepth int
}

func Continue(cont func(Value) Value) Cont {
	return &Continuation{
		Continue: cont,
	}
}

var Identity = Continue(func(v Value) Value {
	return v
})

func (value *Continuation) String() string {
	return "<continuation>"
}

func (value *Continuation) Traced(trace *Trace) Cont {
	cp := *value
	cp.Trace = trace
	cp.TracedDepth++
	return &cp
}

func (value *Continuation) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
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
	case *Value:
		*x = value
		return nil
	default:
		return DecodeError{
			Destination: dest,
			Source:      value,
		}
	}
}

func (value *Continuation) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

func (sink *Continuation) Equal(other Value) bool {
	var o *Continuation
	return other.Decode(&o) == nil && sink == o
}

func (cont *Continuation) Call(res Value, err error) ReadyCont {
	if cont.Trace != nil {
		if err != nil {
			err = TracedError{
				Err:   err,
				Trace: cont.Trace,
			}
		} else {
			cont.Trace.Pop(cont.TracedDepth)
		}
	}

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
	if cont.Err != nil {
		return "<error>"
	} else {
		return fmt.Sprintf("<continue: %s>", cont.Result)
	}
}

func (cont *ReadyContinuation) Go() (Value, error) {
	if cont.Err != nil {
		return nil, cont.Err
	}

	return cont.Continuation.Continue(cont.Result), nil
}

func (value *ReadyContinuation) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *ReadyCont:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	default:
		return DecodeError{
			Destination: dest,
			Source:      value,
		}
	}
}

func (value *ReadyContinuation) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}
