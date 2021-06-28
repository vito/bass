package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestPairDecode(t *testing.T) {
	list := bass.NewList(
		bass.Int(1),
		bass.Bool(true),
		bass.String("three"),
	)

	var dest bass.List
	err := list.Decode(&dest)
	require.NoError(t, err)
	require.Equal(t, list, dest)

	var pair bass.Pair
	err = list.Decode(&pair)
	require.NoError(t, err)
	require.Equal(t, list, pair)
}

func TestPairEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Pair{
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

func TestPairListInterface(t *testing.T) {
	var list bass.List = bass.Pair{bass.Int(1), bass.Bool(true)}
	require.Equal(t, list.First(), bass.Int(1))
	require.Equal(t, list.Rest(), bass.Bool(true))
}

type recorderOp struct {
	Applied bass.Value
	Env     *bass.Env
}

var _ bass.Combiner = recorderOp{}

func (op recorderOp) String() string {
	return "<op: recorder>"
}

func (op recorderOp) Decode(dest interface{}) error {
	switch x := dest.(type) {
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

func (op recorderOp) Eval(*bass.Env) (bass.Value, error) {
	return op, nil
}

func (op recorderOp) Call(val bass.Value, env *bass.Env) (bass.Value, error) {
	op.Applied = val
	op.Env = env
	return op, nil
}
