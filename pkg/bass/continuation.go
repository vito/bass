package bass

import (
	"context"
	"fmt"
	"sync"
)

// Cont is a first-class value representing a continuation.
//
// A continuation is a deferred computation to be called with the result of an
// evaluation. When called it returns a ReadyCont, a "pre-filled" but still
// deferred computation which will utimately be called by the outer trampoline.
//
// Bass must be implemented in continuation-passing style in order to support
// infinite loops. Continuations are however not exposed in the language. They
// are an internal implementation detail which might not be necessary if Bass
// were to be implemented in another host language.
//
// The outer trampoline is simply a loop that calls the returned continuation
// until it stops returning ready continuations and instead returns an inert
// value. This technique is what keeps the stack from growing.
type Cont interface {
	Value

	// Call returns a ReadyCont that will return the given value or error when
	// called by the outer trampoline. The returned value may itself be another
	// ReadyCont.
	Call(Value, error) ReadyCont

	// traced is used to keep track of the actual stack since it is normally lost
	// in the process of using a trampoline.
	traced(*Trace) Cont
}

// ReadyCont is a continuation with a predestined intermediate value or error.
// It is valled by the trampoline.
type ReadyCont interface {
	Value

	// Go either returns the predestined error or calls the inner continuation
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
	return fmt.Sprintf("<continuation: %p>", value)
}

func (value *Continuation) traced(trace *Trace) Cont {
	cp := *value
	cp.Trace = trace
	cp.TracedDepth++
	return &cp
}

func (value *Continuation) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func (value *Continuation) Decode(dest any) error {
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
	New: func() any {
		return &ReadyContinuation{}
	},
}

func (cont *Continuation) Call(res Value, err error) ReadyCont {
	if cont.Trace != nil && err == nil {
		cont.Trace.Pop(cont.TracedDepth)
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

func (value *ReadyContinuation) Decode(dest any) error {
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
