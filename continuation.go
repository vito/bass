package bass

import "fmt"

type Cont interface {
	Value
	Call(Value, error) ReadyCont
}

type ReadyCont interface {
	Value

	Go() (Value, error)
}

type Continuation struct {
	Continue func(Value) Value
	Chain    Cont
}

func Continue(cont func(Value) Value) Cont {
	return &Continuation{
		Continue: cont,
	}
}

func Chain(outer Cont, cont func(Value) Value) Cont {
	return &Continuation{
		Continue: cont,
		Chain:    outer,
	}
}

var Identity = Continue(func(v Value) Value {
	return v
})

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

func (sink *Continuation) Equal(other Value) bool {
	var o *Continuation
	return other.Decode(&o) == nil && sink == o
}

func (cont *Continuation) Call(res Value, err error) ReadyCont {
	if cont.Chain != nil && err != nil {
		// pass err to the original outer continuation to retain trace context
		return cont.Chain.Call(nil, err)
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
