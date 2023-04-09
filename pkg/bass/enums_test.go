package bass_test

import (
	"fmt"
	"reflect"
	"testing"

	. "github.com/vito/bass/pkg/basstest"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/is"
)

type Enum interface {
	FromValue(bass.Value) error
	ToValue() bass.Value
}

func TestEnums(t *testing.T) {
	is := is.New(t)

	type example struct {
		Enum    Enum
		Valid   []bass.Value
		Invalid []bass.Value
	}

	for _, test := range []example{
		{
			Enum: &bass.FileOrDirPath{},
			Valid: []bass.Value{
				bass.FilePath{"file"},
				bass.DirPath{"dir"},
			},
			Invalid: []bass.Value{
				bass.CommandPath{"cmd"},
			},
		},
		{
			Enum: &bass.ThunkDir{},
			Valid: []bass.Value{
				bass.DirPath{"dir"},
				bass.ThunkPath{
					Thunk: bass.Thunk{
						Args: []bass.Value{
							bass.CommandPath{"cmd"},
						},
					},
					Path: bass.FileOrDirPath{
						Dir: &bass.DirPath{"dir"},
					},
				},
			},
			Invalid: []bass.Value{
				bass.CommandPath{"cmd"},
				bass.FilePath{"file"},
				bass.ThunkPath{
					Thunk: bass.Thunk{
						Args: []bass.Value{
							bass.CommandPath{"cmd"},
						},
					},
					Path: bass.FileOrDirPath{
						File: &bass.FilePath{"file"},
					},
				},
			},
		},
		{
			Enum: &bass.ThunkImage{},
			Valid: []bass.Value{
				bass.Bindings{
					"platform": bass.Bindings{
						"os":           bass.String("linux"),
						"architecture": bass.String("amd64"),
					}.Scope(),
					"repository": bass.String("repo"),
				}.Scope(),
				bass.Thunk{
					Args: []bass.Value{
						bass.CommandPath{"cmd"},
					},
				},
			},
			Invalid: []bass.Value{
				bass.String("hello"),
				bass.NewEmptyScope(),
				bass.Null{},
			},
		},
	} {
		test := test

		t.Run(fmt.Sprintf("%T", test.Enum), func(t *testing.T) {
			for _, v := range test.Valid {
				t.Run(fmt.Sprintf("valid: %s", v), func(t *testing.T) {
					is := is.New(t)

					enum := reflect.New(reflect.TypeOf(test.Enum).Elem()).Interface().(Enum)
					err := enum.FromValue(v)
					is.NoErr(err)
					Equal(t, enum.ToValue(), v)
				})
			}

			for _, v := range test.Invalid {
				t.Run(fmt.Sprintf("invalid: %s", v), func(t *testing.T) {
					enum := reflect.New(reflect.TypeOf(test.Enum).Elem()).Interface().(Enum)
					err := enum.FromValue(v)
					is.True(err != nil)
				})
			}
		})
	}
}
