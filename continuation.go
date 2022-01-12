package bass

import (
	"context"
	"fmt"
	"sync"
)

type Cont interface {
	Value

	Call(Value, error) ReadyCont

	Trap(func(error) ReadyCont) Cont
	Traced(*Trace) Cont
}

type ReadyCont interface {
	Value

	Go() (Value, error)
}

type Continuation struct {
	Continue    func(Value) Value
	OnErr       func(error) ReadyCont
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
	return fmt.Sprintf("<continuation: %p>", value)
}

func (value *Continuation) Trap(trap func(error) ReadyCont) Cont {
	cp := *value
	cp.OnErr = trap
	return &cp
}

func (value *Continuation) Traced(trace *Trace) Cont {
	cp := *value
	cp.Trace = trace
	cp.TracedDepth++
	return &cp
}

func (value *Continuation) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
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

func (value *Continuation) Equal(other Value) bool {
	var o *Continuation
	return other.Decode(&o) == nil && value == o
}

var readyContPool = sync.Pool{
	New: func() interface{} {
		return &ReadyContinuation{}
	},
}

func (cont *Continuation) Call(res Value, err error) ReadyCont {
	if cont.Trace != nil && err == nil {
		cont.Trace.Pop(cont.TracedDepth)
	} else if cont.OnErr != nil && err != nil {
		return cont.OnErr(err)
	}

	rc := readyContPool.Get().(*ReadyContinuation)
	rc.Cont = cont
	rc.Result = res
	rc.Err = err
	return rc
}

type ReadyContinuation struct {
	Cont *Continuation

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

func (value *ReadyContinuation) Equal(other Value) bool {
	var o *ReadyContinuation
	return other.Decode(&o) == nil && value == o
}

func (value *ReadyContinuation) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func (cont *ReadyContinuation) Go() (Value, error) {
	defer cont.release()

	if cont.Err != nil {
		return nil, cont.Err
	}

	return cont.Cont.Continue(cont.Result), nil
}

func (cont *ReadyContinuation) release() {
	cont.Cont = nil
	cont.Result = nil
	cont.Err = nil
	readyContPool.Put(cont)
}

func (value *ReadyContinuation) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case **ReadyContinuation:
		*x = value
		return nil
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
