package bass_test

import (
	"context"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/is"
)

func TestBinding(t *testing.T) {
	type example struct {
		Name     string
		Bindable bass.Bindable
		Value    bass.Value

		Bindings bass.Bindings
		Err      error
	}

	for _, test := range []example{
		{
			Name:     "symbol",
			Bindable: bass.Symbol("foo"),
			Value:    bass.String("hello"),
			Bindings: bass.Bindings{
				"foo": bass.String("hello"),
			},
		},
		{
			Name:     "list",
			Bindable: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
			Value:    bass.NewList(bass.Int(1), bass.Int(2)),
			Bindings: bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
			},
		},
		{
			Name:     "empty ok with empty",
			Bindable: bass.Empty{},
			Value:    bass.Empty{},
			Bindings: bass.Bindings{},
		},
		{
			Name:     "empty err on extra",
			Bindable: bass.Empty{},
			Value:    bass.NewList(bass.Int(1), bass.Int(2)),
			Err: bass.BindMismatchError{
				Need: bass.Empty{},
				Have: bass.NewList(bass.Int(1), bass.Int(2)),
			},
		},
		{
			Name:     "list err with empty",
			Bindable: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
			Value:    bass.Empty{},
			Err: bass.BindMismatchError{
				Need: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
				Have: bass.Empty{},
			},
		},
		{
			Name:     "list err with missing value",
			Bindable: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
			Value:    bass.NewList(bass.Int(1)),
			Err: bass.BindMismatchError{
				Need: bass.NewList(bass.Symbol("b")),
				Have: bass.Empty{},
			},
		},
		{
			Name: "pair",
			Bindable: bass.Pair{
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
			Name:     "list with pair",
			Bindable: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
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
			Name:     "unbindable",
			Bindable: bass.NewList(operative),
			Value: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
			Err: bass.CannotBindError{
				Have: operative,
			},
		},
		{
			Name:     "ignore",
			Bindable: bass.Ignore{},
			Value: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
			Bindings: bass.Bindings{},
		},
		{
			Name: "bind and ignore",
			Bindable: bass.Pair{
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
			Name:     "binding ignore",
			Bindable: bass.Symbol("i"),
			Value:    bass.Ignore{},
			Bindings: bass.Bindings{
				"i": bass.Ignore{},
			},
		},
		{
			Name:     "command match",
			Bindable: bass.CommandPath{"foo"},
			Value:    bass.CommandPath{"foo"},
			Bindings: bass.Bindings{},
		},
		{
			Name:     "command mismatch",
			Bindable: bass.CommandPath{"foo"},
			Value:    bass.CommandPath{"bar"},
			Err: bass.BindMismatchError{
				Need: bass.CommandPath{"foo"},
				Have: bass.CommandPath{"bar"},
			},
		},
		{
			Name:     "file match",
			Bindable: bass.FilePath{"foo"},
			Value:    bass.FilePath{"foo"},
			Bindings: bass.Bindings{},
		},
		{
			Name:     "file mismatch",
			Bindable: bass.FilePath{"foo"},
			Value:    bass.FilePath{"bar"},
			Err: bass.BindMismatchError{
				Need: bass.FilePath{"foo"},
				Have: bass.FilePath{"bar"},
			},
		},
		{
			Name:     "dir match",
			Bindable: bass.NewDir("foo"),
			Value:    bass.NewDir("foo"),
			Bindings: bass.Bindings{},
		},
		{
			Name:     "dir mismatch",
			Bindable: bass.NewDir("foo"),
			Value:    bass.NewDir("bar"),
			Err: bass.BindMismatchError{
				Need: bass.NewDir("foo"),
				Have: bass.NewDir("bar"),
			},
		},
		{
			Name:     "null match",
			Bindable: bass.Null{},
			Value:    bass.Null{},
			Bindings: bass.Bindings{},
		},
		{
			Name:     "null mismatch",
			Bindable: bass.Null{},
			Value:    bass.Bool(false),
			Err: bass.BindMismatchError{
				Need: bass.Null{},
				Have: bass.Bool(false),
			},
		},
		{
			Name:     "bool match",
			Bindable: bass.Bool(true),
			Value:    bass.Bool(true),
			Bindings: bass.Bindings{},
		},
		{
			Name:     "bool mismatch",
			Bindable: bass.Bool(true),
			Value:    bass.Bool(false),
			Err: bass.BindMismatchError{
				Need: bass.Bool(true),
				Have: bass.Bool(false),
			},
		},
		{
			Name:     "int match",
			Bindable: bass.Int(42),
			Value:    bass.Int(42),
			Bindings: bass.Bindings{},
		},
		{
			Name:     "int mismatch",
			Bindable: bass.Int(42),
			Value:    bass.Int(24),
			Err: bass.BindMismatchError{
				Need: bass.Int(42),
				Have: bass.Int(24),
			},
		},
		{
			Name:     "string match",
			Bindable: bass.String("hello"),
			Value:    bass.String("hello"),
			Bindings: bass.Bindings{},
		},
		{
			Name:     "string mismatch",
			Bindable: bass.String("hello"),
			Value:    bass.String("goodbye"),
			Err: bass.BindMismatchError{
				Need: bass.String("hello"),
				Have: bass.String("goodbye"),
			},
		},
		{
			Name:     "keyword match symbol",
			Bindable: bass.Keyword("hello"),
			Value:    bass.Symbol("hello"),
			Bindings: bass.Bindings{},
		},
		{
			Name:     "keyword mismatch symbol",
			Bindable: bass.Keyword("hello"),
			Value:    bass.Symbol("goodbye"),
			Err: bass.BindMismatchError{
				Need: bass.Symbol("hello"),
				Have: bass.Symbol("goodbye"),
			},
		},
		{
			Name:     "keyword mismatch keyword",
			Bindable: bass.Keyword("hello"),
			Value:    bass.Keyword("hello"),
			Err: bass.BindMismatchError{
				Need: bass.Symbol("hello"),
				Have: bass.Keyword("hello"),
			},
		},
		{
			Name:     "binding bind empty",
			Bindable: bass.Bind{},
			Value:    bass.NewEmptyScope(),
			Bindings: bass.Bindings{},
		},
		{
			Name:     "binding bind extra values",
			Bindable: bass.Bind{},
			Value:    bass.Bindings{"extra": bass.Bool(true)}.Scope(),
			Bindings: bass.Bindings{},
		},
		{
			Name: "binding bind",
			Bindable: bass.Bind{
				bass.Keyword("foo"), bass.Symbol("foo-bnd"),
				bass.Keyword("bar"), bass.Symbol("bar-bnd"),
			},
			Value: bass.Bindings{
				"foo": bass.Bool(true),
				"bar": bass.Int(42),
			}.Scope(),
			Bindings: bass.Bindings{
				"foo-bnd": bass.Bool(true),
				"bar-bnd": bass.Int(42),
			},
		},
		{
			Name: "binding bind unbound",
			Bindable: bass.Bind{
				bass.Keyword("foo"), bass.Symbol("foo-bnd"),
				bass.Keyword("bar"), bass.Symbol("bar-bnd"),
			},
			Value: bass.Bindings{
				"bar": bass.Int(42),
			}.Scope(),
			Err: bass.UnboundError{
				Symbol: "foo",
				Scope: bass.Bindings{
					"bar": bass.Int(42),
				}.Scope(),
			},
		},
		{
			Name: "binding bind mismatch",
			Bindable: bass.Bind{
				bass.Keyword("foo"), bass.Symbol("foo-bnd"),
				bass.Symbol("bar-bnd"),
			},
			Value: bass.Bindings{
				"bar": bass.Int(42),
			}.Scope(),
			Err: bass.ErrBadSyntax,
		},
		{
			Name: "binding bind default",
			Bindable: bass.Bind{
				bass.NewList(bass.Keyword("foo"), bass.Symbol("sentinel")), bass.Symbol("foo-bnd"),
				bass.Keyword("bar"), bass.Symbol("bar-bnd"),
			},
			Value: bass.Bindings{
				"bar": bass.Int(42),
			}.Scope(),
			Bindings: bass.Bindings{
				"foo-bnd": bass.Symbol("evaluated"),
				"bar-bnd": bass.Int(42),
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			is := is.New(t)

			parent := bass.Bindings{"sentinel": bass.Symbol("evaluated")}.Scope()
			scope := bass.NewEmptyScope(parent)

			ctx := context.Background()

			_, err := bass.Trampoline(ctx, test.Bindable.Bind(ctx, scope, bass.Identity, test.Value))
			if test.Err != nil {
				is.Equal(err, test.Err)
			} else {
				is.NoErr(err)
				is.Equal(scope.Bindings, test.Bindings)
			}
		})
	}
}
