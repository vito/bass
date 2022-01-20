package bass_test

import (
	"context"
	"testing"

	"github.com/vito/bass/bass"
	"github.com/vito/is"
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
			Name:   "unbindable",
			Params: bass.NewList(operative),
			Value: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
			Err: bass.CannotBindError{
				Have: operative,
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
		{
			Name:     "command match",
			Params:   bass.CommandPath{"foo"},
			Value:    bass.CommandPath{"foo"},
			Bindings: bass.Bindings{},
		},
		{
			Name:   "command mismatch",
			Params: bass.CommandPath{"foo"},
			Value:  bass.CommandPath{"bar"},
			Err: bass.BindMismatchError{
				Need: bass.CommandPath{"foo"},
				Have: bass.CommandPath{"bar"},
			},
		},
		{
			Name:     "file match",
			Params:   bass.FilePath{"foo"},
			Value:    bass.FilePath{"foo"},
			Bindings: bass.Bindings{},
		},
		{
			Name:   "file mismatch",
			Params: bass.FilePath{"foo"},
			Value:  bass.FilePath{"bar"},
			Err: bass.BindMismatchError{
				Need: bass.FilePath{"foo"},
				Have: bass.FilePath{"bar"},
			},
		},
		{
			Name:     "dir match",
			Params:   bass.DirPath{"foo"},
			Value:    bass.DirPath{"foo"},
			Bindings: bass.Bindings{},
		},
		{
			Name:   "dir mismatch",
			Params: bass.DirPath{"foo"},
			Value:  bass.DirPath{"bar"},
			Err: bass.BindMismatchError{
				Need: bass.DirPath{"foo"},
				Have: bass.DirPath{"bar"},
			},
		},
		{
			Name:     "null match",
			Params:   bass.Null{},
			Value:    bass.Null{},
			Bindings: bass.Bindings{},
		},
		{
			Name:   "null mismatch",
			Params: bass.Null{},
			Value:  bass.Bool(false),
			Err: bass.BindMismatchError{
				Need: bass.Null{},
				Have: bass.Bool(false),
			},
		},
		{
			Name:     "bool match",
			Params:   bass.Bool(true),
			Value:    bass.Bool(true),
			Bindings: bass.Bindings{},
		},
		{
			Name:   "bool mismatch",
			Params: bass.Bool(true),
			Value:  bass.Bool(false),
			Err: bass.BindMismatchError{
				Need: bass.Bool(true),
				Have: bass.Bool(false),
			},
		},
		{
			Name:     "int match",
			Params:   bass.Int(42),
			Value:    bass.Int(42),
			Bindings: bass.Bindings{},
		},
		{
			Name:   "int mismatch",
			Params: bass.Int(42),
			Value:  bass.Int(24),
			Err: bass.BindMismatchError{
				Need: bass.Int(42),
				Have: bass.Int(24),
			},
		},
		{
			Name:     "string match",
			Params:   bass.String("hello"),
			Value:    bass.String("hello"),
			Bindings: bass.Bindings{},
		},
		{
			Name:   "string mismatch",
			Params: bass.String("hello"),
			Value:  bass.String("goodbye"),
			Err: bass.BindMismatchError{
				Need: bass.String("hello"),
				Have: bass.String("goodbye"),
			},
		},
		{
			Name:     "keyword match",
			Params:   bass.Keyword("hello"),
			Value:    bass.Keyword("hello"),
			Bindings: bass.Bindings{},
		},
		{
			Name:   "keyword mismatch",
			Params: bass.Keyword("hello"),
			Value:  bass.Keyword("goodbye"),
			Err: bass.BindMismatchError{
				Need: bass.Keyword("hello"),
				Have: bass.Keyword("goodbye"),
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			is := is.New(t)

			scope := bass.NewEmptyScope()

			ctx := context.Background()

			_, err := bass.Trampoline(ctx, test.Params.Bind(ctx, scope, bass.Identity, test.Value))
			if test.Err != nil {
				is.Equal(err, test.Err)
			} else {
				is.NoErr(err)
				is.Equal(scope.Bindings, test.Bindings)
			}
		})
	}
}
