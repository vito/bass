package bass_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func Equal(t *testing.T, a, b bass.Value) {
	require.True(t, a.Equal(b), "%s != %s", a, b)
}

func Eval(env *bass.Env, val bass.Value) (bass.Value, error) {
	return EvalContext(context.Background(), env, val)
}

func EvalContext(ctx context.Context, env *bass.Env, val bass.Value) (bass.Value, error) {
	return bass.Trampoline(ctx, val.Eval(ctx, env, bass.Identity))
}

func Call(comb bass.Combiner, env *bass.Env, val bass.Value) (bass.Value, error) {
	ctx := context.Background()
	return bass.Trampoline(ctx, comb.Call(ctx, val, env, bass.Identity))
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

func (op recorderOp) Eval(ctx context.Context, env *bass.Env, cont bass.Cont) bass.ReadyCont {
	return cont.Call(op, nil)
}

func (op recorderOp) Call(ctx context.Context, val bass.Value, env *bass.Env, cont bass.Cont) bass.ReadyCont {
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

func (val dummyValue) Eval(ctx context.Context, env *bass.Env, cont bass.Cont) bass.ReadyCont {
	return cont.Call(val, nil)
}

type wrappedValue struct {
	bass.Value
}

type Const struct {
	bass.Value
}

func (value Const) Eval(ctx context.Context, env *bass.Env, cont bass.Cont) bass.ReadyCont {
	return cont.Call(value.Value, nil)
}

type dummyPath struct {
	dummyValue

	extended bass.Path
}

func (path *dummyPath) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *bass.Value:
		*x = path
		return nil
	case *bass.Path:
		*x = path
		return nil
	default:
		return bass.DecodeError{
			Source:      path,
			Destination: dest,
		}
	}
}

func (path *dummyPath) Eval(ctx context.Context, env *bass.Env, cont bass.Cont) bass.ReadyCont {
	return cont.Call(path, nil)
}

func (path *dummyPath) Resolve(root string) (string, error) {
	return "resolved", nil
}

func (path *dummyPath) Extend(sub bass.Path) (bass.Path, error) {
	path.extended = sub
	return path, nil
}

func (value *dummyPath) FromObject(obj bass.Object) error {
	return fmt.Errorf("dummyPath FromObject unimplemented")
}
