package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestBinding(t *testing.T) {
	type example struct {
		Name   string
		Params bass.Bindable
		Value  bass.Value

		Bindings bass.Bindings
		Err      error
	}

	for _, test := range []example{
		{
			Name:   "symbol",
			Params: bass.Symbol("foo"),
			Value:  bass.String("hello"),
			Bindings: bass.Bindings{
				"foo": bass.String("hello"),
			},
		},
		{
			Name:   "list",
			Params: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
			Value:  bass.NewList(bass.Int(1), bass.Int(2)),
			Bindings: bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
			},
		},
		{
			Name:     "empty ok with empty",
			Params:   bass.Empty{},
			Value:    bass.Empty{},
			Bindings: bass.Bindings{},
		},
		{
			Name:   "empty err on extra",
			Params: bass.Empty{},
			Value:  bass.NewList(bass.Int(1), bass.Int(2)),
			Err: bass.BindMismatchError{
				Need: bass.Empty{},
				Have: bass.NewList(bass.Int(1), bass.Int(2)),
			},
		},
		{
			Name:   "list err with empty",
			Params: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
			Value:  bass.Empty{},
			Err: bass.BindMismatchError{
				Need: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
				Have: bass.Empty{},
			},
		},
		{
			Name:   "list err with missing value",
			Params: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
			Value:  bass.NewList(bass.Int(1)),
			Err: bass.BindMismatchError{
				Need: bass.NewList(bass.Symbol("b")),
				Have: bass.Empty{},
			},
		},
		{
			Name: "pair",
			Params: bass.Pair{
				A: bass.Symbol("a"),
				D: bass.Symbol("d"),
			},
			Value: bass.NewList(bass.Int(1), bass.Int(2)),
			Bindings: bass.Bindings{
				"a": bass.Int(1),
				"d": bass.NewList(bass.Int(2)),
			},
		},
		{
			Name:   "list with pair",
			Params: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
			Value: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
			Err: bass.BindMismatchError{
				Need: bass.NewList(bass.Symbol("b")),
				Have: bass.Int(2),
			},
		},
		{
			Name:   "unassignable",
			Params: bass.NewList(bass.Int(1)),
			Value: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
			Err: bass.CannotBindError{
				Have: bass.Int(1),
			},
		},
		{
			Name:   "ignore",
			Params: bass.Ignore{},
			Value: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
			Bindings: bass.Bindings{},
		},
		{
			Name: "bind and ignore",
			Params: bass.Pair{
				A: bass.Ignore{},
				D: bass.Symbol("b"),
			},
			Value: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
			Bindings: bass.Bindings{
				"b": bass.Int(2),
			},
		},
		{
			Name:   "binding ignore",
			Params: bass.Symbol("i"),
			Value:  bass.Ignore{},
			Bindings: bass.Bindings{
				"i": bass.Ignore{},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			env := bass.NewEnv()

			err := test.Params.Bind(env, test.Value)
			if test.Err != nil {
				require.Equal(t, test.Err, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.Bindings, env.Bindings)
			}
		})
	}
}
