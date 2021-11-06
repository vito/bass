package bass_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spy16/slurp/reader"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/is"
)

func TestConstsEval(t *testing.T) {
	scope := bass.NewEmptyScope()

	for _, val := range allConstValues {
		t.Run(val.String(), func(t *testing.T) {
			is := is.New(t)
			res, err := Eval(scope, val)
			is.NoErr(err)
			is.True(val.Equal(res))
		})
	}
}

func TestKeywordEval(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	val := bass.Keyword("foo")

	res, err := Eval(scope, val)
	is.NoErr(err)
	is.Equal(res, bass.Symbol("foo"))
}

func TestSymbolEval(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	val := bass.Symbol("foo")

	_, err := Eval(scope, val)
	is.Equal(err, bass.UnboundError{"foo"})

	scope.Set(val, bass.Int(42))

	res, err := Eval(scope, val)
	is.NoErr(err)
	is.Equal(res, bass.Int(42))
}

func TestPairEval(t *testing.T) {
	is := is.New(t)

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
	is.NoErr(err)
	is.Equal(

		res, recorderOp{
			Applied: bass.Pair{
				A: bass.Int(42),
				D: bass.Pair{
					A: bass.Symbol("unevaluated"),
					D: bass.Empty{},
				},
			},
			Scope: scope,
		})

}

func TestConsEval(t *testing.T) {
	is := is.New(t)

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
	is.NoErr(err)
	is.Equal(res, expected)
}

func TestBindEval(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	val := bass.Bind{
		bass.Keyword("a"), bass.Int(1),
		bass.Symbol("key"), bass.Bool(true),
		bass.Keyword("c"), bass.Symbol("value"),
	}

	scope.Set("key", bass.Symbol("b"))
	scope.Set("value", bass.String("three"))

	res, err := Eval(scope, val)
	is.NoErr(err)
	is.Equal(

		res, bass.NewScope(bass.Bindings{
			"a": bass.Int(1),
			"b": bass.Bool(true),
			"c": bass.String("three"),
		}))

	scope.Set("key", bass.String("non-key"))

	_, err = Eval(scope, val)
	is.True(errors.Is(err, bass.ErrBadSyntax))
}

func TestAnnotatedEval(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	scope.Set(bass.Symbol("foo"), bass.Symbol("bar"))

	val := bass.Annotated{
		Comment: "hello",
		Value:   bass.Symbol("foo"),
	}

	res, err := Eval(scope, val)
	is.NoErr(err)
	is.Equal(res, bass.Symbol("bar"))

	is.True(len(scope.Commentary) > 0)
	is.Equal(scope.Commentary, []bass.Annotated{
		{
			Comment: "hello",
			Value:   bass.Symbol("bar"),
		},
	})

	doc, found := scope.GetDoc("bar")
	is.True(found)
	is.Equal(bass.Annotated{
		Comment: "hello",
		Value:   bass.Symbol("bar"),
	}, doc)

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
	is.Equal(bass.UnboundError{"unknown"}, err)
}

func TestExtendPathEval(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	dummy := &dummyPath{}

	val := bass.ExtendPath{
		dummy,
		bass.FilePath{"foo"},
	}

	res, err := Eval(scope, val)
	is.NoErr(err)
	is.Equal(res, dummy)
	is.Equal(bass.FilePath{"foo"}, dummy.extended)
}
