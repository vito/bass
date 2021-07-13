package bass_test

import (
	"testing"

	"github.com/spy16/slurp/reader"
	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestConstsEval(t *testing.T) {
	env := bass.NewEnv()

	for _, val := range allConstValues {
		t.Run(val.String(), func(t *testing.T) {
			res, err := Eval(env, val)
			require.NoError(t, err)
			Equal(t, val, res)
		})
	}
}

func TestSymbolEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Symbol("foo")

	_, err := Eval(env, val)
	require.Equal(t, bass.UnboundError{"foo"}, err)

	env.Set(val, bass.Int(42))

	res, err := Eval(env, val)
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)
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

	res, err := Eval(env, val)
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

func TestConsEval(t *testing.T) {
	env := bass.NewEnv()

	env.Set("foo", bass.String("hello"))
	env.Set("bar", bass.String("world"))

	val := bass.Cons{
		A: bass.Symbol("foo"),
		D: bass.Cons{
			A: bass.Symbol("bar"),
			D: bass.Empty{},
		},
	}

	expected := bass.Pair{
		A: bass.String("hello"),
		D: bass.Pair{
			A: bass.String("world"),
			D: bass.Empty{},
		},
	}

	res, err := Eval(env, val)
	require.NoError(t, err)
	require.Equal(t, expected, res)
}

func TestAssocEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.Assoc{
		{bass.Keyword("a"), bass.Int(1)},
		{bass.Symbol("key"), bass.Bool(true)},
		{bass.Keyword("c"), bass.Symbol("value")},
	}

	env.Set("key", bass.Keyword("b"))
	env.Set("value", bass.String("three"))

	res, err := Eval(env, val)
	require.NoError(t, err)
	require.Equal(t, bass.Object{
		"a": bass.Int(1),
		"b": bass.Bool(true),
		"c": bass.String("three"),
	}, res)

	env.Set("key", bass.String("non-key"))

	res, err = Eval(env, val)
	require.ErrorIs(t, err, bass.BadKeyError{
		Value: bass.String("non-key"),
	})
}

func TestAnnotatedEval(t *testing.T) {
	env := bass.NewEnv()
	env.Set(bass.Symbol("foo"), bass.Symbol("bar"))

	val := bass.Annotated{
		Comment: "hello",
		Value:   bass.Symbol("foo"),
	}

	res, err := Eval(env, val)
	require.NoError(t, err)
	require.Equal(t, bass.Symbol("bar"), res)

	require.NotEmpty(t, env.Commentary)
	require.ElementsMatch(t, env.Commentary, []bass.Value{
		bass.Annotated{
			Comment: "hello",
			Value:   bass.Symbol("bar"),
		},
	})
	require.Equal(t, env.Docs, bass.Docs{
		"bar": "hello",
	})

	loc := bass.Range{
		Start: reader.Position{
			File: "some-file",
			Ln:   42,
			Col:  12,
		},
		End: reader.Position{
			File: "some-file",
			Ln:   44,
			Col:  22,
		},
	}

	val = bass.Annotated{
		Value: bass.Symbol("unknown"),
		Range: loc,
	}

	_, err = Eval(env, val)
	require.ErrorIs(t, err, bass.UnboundError{"unknown"})
	require.ErrorIs(t, err, bass.AnnotatedError{
		Value: bass.Symbol("unknown"),
		Range: loc,
		Err:   bass.UnboundError{"unknown"},
	})
}

func TestExtendPathEval(t *testing.T) {
	env := bass.NewEnv()
	dummy := &dummyPath{}

	val := bass.ExtendPath{
		dummy,
		bass.FilePath{"foo"},
	}

	res, err := Eval(env, val)
	require.NoError(t, err)
	require.Equal(t, dummy, res)
	require.Equal(t, dummy.extended, bass.FilePath{"foo"})
}
