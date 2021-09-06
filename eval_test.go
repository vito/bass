package bass_test

import (
	"context"
	"testing"

	"github.com/spy16/slurp/reader"
	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
)

func TestConstsEval(t *testing.T) {
	scope := bass.NewEmptyScope()

	for _, val := range allConstValues {
		t.Run(val.String(), func(t *testing.T) {
			res, err := Eval(scope, val)
			require.NoError(t, err)
			Equal(t, val, res)
		})
	}
}

func TestKeywordEval(t *testing.T) {
	scope := bass.NewEmptyScope()
	val := bass.Keyword("foo")

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, bass.Symbol("foo"), res)
}

func TestSymbolEval(t *testing.T) {
	scope := bass.NewEmptyScope()
	val := bass.Symbol("foo")

	_, err := Eval(scope, val)
	require.Equal(t, bass.UnboundError{"foo"}, err)

	scope.Set(val, bass.Int(42))

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)
}

func TestPairEval(t *testing.T) {
	scope := bass.NewEmptyScope()
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

	scope.Set("foo", recorderOp{})

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, recorderOp{
		Applied: bass.Pair{
			A: bass.Int(42),
			D: bass.Pair{
				A: bass.Symbol("unevaluated"),
				D: bass.Empty{},
			},
		},
		Scope: scope,
	}, res)
}

func TestConsEval(t *testing.T) {
	scope := bass.NewEmptyScope()

	scope.Set("foo", bass.String("hello"))
	scope.Set("bar", bass.String("world"))

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

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, expected, res)
}

func TestBindEval(t *testing.T) {
	scope := bass.NewEmptyScope()
	val := bass.Bind{
		bass.Keyword("a"), bass.Int(1),
		bass.Symbol("key"), bass.Bool(true),
		bass.Keyword("c"), bass.Symbol("value"),
	}

	scope.Set("key", bass.Symbol("b"))
	scope.Set("value", bass.String("three"))

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, bass.NewScope(bass.Bindings{
		"a": bass.Int(1),
		"b": bass.Bool(true),
		"c": bass.String("three"),
	}), res)

	scope.Set("key", bass.String("non-key"))

	_, err = Eval(scope, val)
	require.ErrorIs(t, err, bass.ErrBadSyntax)
}

func TestAnnotatedEval(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Set(bass.Symbol("foo"), bass.Symbol("bar"))

	val := bass.Annotated{
		Comment: "hello",
		Value:   bass.Symbol("foo"),
	}

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, bass.Symbol("bar"), res)

	require.NotEmpty(t, scope.Commentary)
	require.ElementsMatch(t, scope.Commentary, []bass.Value{
		bass.Annotated{
			Comment: "hello",
			Value:   bass.Symbol("bar"),
		},
	})

	doc, found := scope.GetDoc("bar")
	require.True(t, found)
	require.Equal(t, doc, bass.Annotated{
		Comment: "hello",
		Value:   bass.Symbol("bar"),
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

	trace := &bass.Trace{}
	ctx := bass.WithTrace(context.Background(), trace)

	_, err = EvalContext(ctx, scope, val)
	require.Equal(t, err, bass.UnboundError{"unknown"})
}

func TestExtendPathEval(t *testing.T) {
	scope := bass.NewEmptyScope()
	dummy := &dummyPath{}

	val := bass.ExtendPath{
		dummy,
		bass.FilePath{"foo"},
	}

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, dummy, res)
	require.Equal(t, dummy.extended, bass.FilePath{"foo"})
}
