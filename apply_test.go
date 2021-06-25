package bass_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestApplyEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Apply{
		A: bass.Symbol("foo"),
		D: bass.Pair{
			A: bass.Int(42),
			D: bass.Pair{
				A: bass.Symbol("unevaluated"),
				D: bass.Empty{},
			},
		},
	}

	env.Set("foo", recorderOp{})

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, recorderOp{
		Applied: bass.Pair{
			A: bass.Int(42),
			D: bass.Pair{
				A: bass.Symbol("unevaluated"),
				D: bass.Empty{},
			},
		},
		Env: env,
	}, res)
}

type recorderOp struct {
	Applied bass.Value
	Env     *bass.Env
}

var _ bass.Combiner = recorderOp{}

func (op recorderOp) Decode(interface{}) error {
	return fmt.Errorf("unimplemented")
}

func (op recorderOp) Eval(*bass.Env) (bass.Value, error) {
	return op, nil
}

func (op recorderOp) Call(val bass.Value, env *bass.Env) (bass.Value, error) {
	op.Applied = val
	op.Env = env
	return op, nil
}
