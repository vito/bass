package bass

import "fmt"

type Cont interface {
	Value
	Call(Value) ReadyCont
}

type ReadyCont interface {
	Value

	Go() (Value, error)
}

type Continuation struct {
	Continue func(Value) (Value, error)
}

func Continue(cont func(Value) (Value, error)) Cont {
	return &Continuation{
		Continue: cont,
	}
}

var Identity = Continue(func(v Value) (Value, error) {
	return v, nil
})

func (value *Continuation) String() string {
	return "<continuation>"
}

func (value *Continuation) Eval(env *Env, cont Cont) (ReadyCont, error) {
	return cont.Call(value), nil
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

func (cont *Continuation) Call(res Value) ReadyCont {
	return &ReadyContinuation{
		Continuation: cont,
		Result:       res,
	}
}

type ReadyContinuation struct {
	*Continuation

	Result Value
}

func (cont *ReadyContinuation) String() string {
	return fmt.Sprintf("<continue: %s>", cont.Result)
}

func (cont *ReadyContinuation) Go() (Value, error) {
	return cont.Continuation.Continue(cont.Result)
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
