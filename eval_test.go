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
	val := bass.Keyword(bass.NewSymbol("foo"))

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, bass.NewSymbol("foo"), res)
}

func TestSymbolEval(t *testing.T) {
	scope := bass.NewEmptyScope()
	val := bass.NewSymbol("foo")

	_, err := Eval(scope, val)
	require.Equal(t, bass.UnboundError{val}, err)

	scope.Set(val, bass.Int(42))

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, bass.Int(42), res)
}

func TestPairEval(t *testing.T) {
	scope := bass.NewEmptyScope()
	val := bass.Pair{
		A: bass.NewSymbol("foo"),
		D: bass.Pair{
			A: bass.Int(42),
			D: bass.Pair{
				A: bass.NewSymbol("unevaluated"),
				D: bass.Empty{},
			},
		},
	}

	scope.Def("foo", recorderOp{})

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, recorderOp{
		Applied: bass.Pair{
			A: bass.Int(42),
			D: bass.Pair{
				A: bass.NewSymbol("unevaluated"),
				D: bass.Empty{},
			},
		},
		Scope: scope,
	}, res)
}

func TestConsEval(t *testing.T) {
	scope := bass.NewEmptyScope()

	scope.Def("foo", bass.String("hello"))
	scope.Def("bar", bass.String("world"))

	val := bass.Cons{
		A: bass.NewSymbol("foo"),
		D: bass.Cons{
			A: bass.NewSymbol("bar"),
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
		bass.NewKeyword("a"), bass.Int(1),
		bass.NewSymbol("key"), bass.Bool(true),
		bass.NewKeyword("c"), bass.NewSymbol("value"),
	}

	scope.Def("key", bass.NewSymbol("b"))
	scope.Def("value", bass.String("three"))

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, bass.Bindings{
		"a": bass.Int(1),
		"b": bass.Bool(true),
		"c": bass.String("three"),
	}.Scope(), res)

	scope.Def("key", bass.String("non-key"))

	_, err = Eval(scope, val)
	require.ErrorIs(t, err, bass.ErrBadSyntax)
}

func TestAnnotatedEval(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Set(bass.NewSymbol("foo"), bass.NewSymbol("bar"))

	val := bass.Annotated{
		Comment: "hello",
		Value:   bass.NewSymbol("foo"),
	}

	res, err := Eval(scope, val)
	require.NoError(t, err)
	require.Equal(t, bass.NewSymbol("bar"), res)

	require.NotEmpty(t, scope.Commentary)
	require.ElementsMatch(t, scope.Commentary, []bass.Value{
		bass.Annotated{
			Comment: "hello",
			Value:   bass.NewSymbol("bar"),
		},
	})

	doc, found := scope.GetDoc(bass.NewSymbol("bar"))
	require.True(t, found)
	require.Equal(t, doc, bass.Annotated{
		Comment: "hello",
		Value:   bass.NewSymbol("bar"),
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
		Value: bass.NewSymbol("unknown"),
		Range: loc,
	}

	trace := &bass.Trace{}
	ctx := bass.WithTrace(context.Background(), trace)

	_, err = EvalContext(ctx, scope, val)
	require.Equal(t, err, bass.UnboundError{bass.NewSymbol("unknown")})
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
