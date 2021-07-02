package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func Equal(t *testing.T, a, b bass.Value) {
	require.True(t, a.Equal(b), "%s != %s", a, b)
}

func Eval(env *bass.Env, val bass.Value) (bass.Value, error) {
	rdy := val.Eval(env, bass.Identity)

	return bass.Trampoline(rdy)
}

func Call(comb bass.Combiner, env *bass.Env, val bass.Value) (bass.Value, error) {
	rdy := comb.Call(val, env, bass.Identity)

	return bass.Trampoline(rdy)
}

type recorderOp struct {
	Applied bass.Value
	Env     *bass.Env
}

var _ bass.Combiner = recorderOp{}

func (op recorderOp) String() string {
	return "<op: recorder>"
}

func (op recorderOp) Equal(other bass.Value) bool {
	var o recorderOp
	if err := other.Decode(&o); err != nil {
		return false
	}

	return op == o
}

func (op recorderOp) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *recorderOp:
		*x = op
		return nil
	case *bass.Combiner:
		*x = op
		return nil
	default:
		return bass.DecodeError{
			Source:      op,
			Destination: dest,
		}
	}
}

func (op recorderOp) Eval(env *bass.Env, cont bass.Cont) bass.ReadyCont {
	return cont.Call(op, nil)
}

func (op recorderOp) Call(val bass.Value, env *bass.Env, cont bass.Cont) bass.ReadyCont {
	op.Applied = val
	op.Env = env
	return cont.Call(op, nil)
}

type dummyValue struct {
	sentinel int
}

var _ bass.Value = dummyValue{}

func (val dummyValue) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *dummyValue:
		*x = val
		return nil
	default:
		return bass.DecodeError{
			Source:      val,
			Destination: dest,
		}
	}
}

func (val dummyValue) Equal(other bass.Value) bool {
	var o dummyValue
	if err := other.Decode(&o); err != nil {
		return false
	}

	return val.sentinel == o.sentinel
}

func (dummyValue) String() string { return "<dummy>" }

func (val dummyValue) Eval(env *bass.Env, cont bass.Cont) bass.ReadyCont {
	return cont.Call(val, nil)
}

type wrappedValue struct {
	bass.Value
}

type Const struct {
	bass.Value
}

func (value Const) Eval(env *bass.Env, cont bass.Cont) bass.ReadyCont {
	return cont.Call(value.Value, nil)
}
