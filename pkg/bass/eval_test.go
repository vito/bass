package bass_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
)

func TestConstsEval(t *testing.T) {
	scope := bass.NewEmptyScope()

	for _, val := range allConstValues {
		t.Run(val.String(), func(t *testing.T) {
			is := is.New(t)
			res, err := Eval(scope, val)
			is.NoErr(err)
			is.Equal(val, res)
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
	is.Equal(err, bass.UnboundError{"foo", scope})

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
	Equal(t, res, bass.NewScope(bass.Bindings{
		"a": bass.Int(1),
		"b": bass.Bool(true),
		"c": bass.String("three"),
	}))

	scope.Set("key", bass.String("non-key"))

	_, err = Eval(scope, val)
	is.True(errors.Is(err, bass.ErrBadSyntax))
}

func TestAnnotateEval(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	scope.Set(bass.Symbol("foo"), bass.Symbol("bar"))

	form := bass.Annotate{
		Comment: "hello",
		Value:   bass.Symbol("foo"),
	}

	res, err := Eval(scope, form)
	is.NoErr(err)

	var ann bass.Annotated
	is.NoErr(res.Decode(&ann))
	is.Equal(ann.Value, bass.Symbol("bar"))
	var doc string
	is.NoErr(ann.Meta.GetDecode("doc", &doc))
	is.Equal(doc, "hello")

	_, found := scope.Get("bar")
	is.True(!found) // binding isn't actually set; it only exists in commentary

	loc := bass.Range{
		File: bass.NewInMemoryFile("test", ""),
		Start: bass.Position{
			Ln:  42,
			Col: 12,
		},
		End: bass.Position{
			Ln:  44,
			Col: 22,
		},
	}

	form = bass.Annotate{
		Value: bass.Symbol("unknown"),
		Range: loc,
	}

	trace := &bass.Trace{}
	ctx := bass.WithTrace(context.Background(), trace)

	_, err = EvalContext(ctx, scope, form)
	is.Equal(bass.UnboundError{"unknown", scope}, err)
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
	is.Equal(dummy, res)
	is.Equal(bass.FilePath{"foo"}, dummy.extended)
}

type dummyPath struct {
	val dummyValue

	extended bass.Path
}

func (path *dummyPath) String() string {
	return fmt.Sprintf("<dummy-path: %s/%s>", path.val, path.extended)
}

func (path *dummyPath) Equal(other bass.Value) bool {
	return reflect.DeepEqual(path, other)
}

func (path *dummyPath) Decode(dest any) error {
	switch x := dest.(type) {
	case *bass.Value:
		*x = path
		return nil
	case *bass.Combiner:
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

func (path *dummyPath) Call(ctx context.Context, val bass.Value, scope *bass.Scope, cont bass.Cont) bass.ReadyCont {
	return bass.Wrap(bass.ExtendOperative{path}).Call(ctx, val, scope, cont)
}

func (path *dummyPath) Eval(_ context.Context, _ *bass.Scope, cont bass.Cont) bass.ReadyCont {
	return cont.Call(path, nil)
}

func (path *dummyPath) Name() string {
	return fmt.Sprintf("<dummy name: %s>", path)
}

func (path *dummyPath) Extend(sub bass.Path) (bass.Path, error) {
	path.extended = sub
	return path, nil
}
