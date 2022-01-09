package bass_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	. "github.com/vito/bass/basstest"

	"github.com/vito/bass"
	"github.com/vito/is"
)

type Enum interface {
	FromValue(bass.Value) error
	ToValue() bass.Value

	json.Marshaler
	json.Unmarshaler
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
			Enum: &bass.ThunkRunPath{},
			Valid: []bass.Value{
				bass.CommandPath{"cmd"},
				bass.FilePath{"file"},
				bass.ThunkPath{
					Thunk: bass.Thunk{
						Path: bass.ThunkRunPath{
							Cmd: &bass.CommandPath{"cmd"},
						},
					},
					Path: bass.FileOrDirPath{
						File: &bass.FilePath{"file"},
					},
				},
			},
			Invalid: []bass.Value{
				bass.DirPath{"dir"},
				bass.ThunkPath{
					Thunk: bass.Thunk{
						Path: bass.ThunkRunPath{
							Cmd: &bass.CommandPath{"cmd"},
						},
					},
					Path: bass.FileOrDirPath{
						Dir: &bass.DirPath{"dir"},
					},
				},
			},
		},
		{
			Enum: &bass.ThunkRunDir{},
			Valid: []bass.Value{
				bass.DirPath{"dir"},
				bass.ThunkPath{
					Thunk: bass.Thunk{
						Path: bass.ThunkRunPath{
							Cmd: &bass.CommandPath{"cmd"},
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
						Path: bass.ThunkRunPath{
							Cmd: &bass.CommandPath{"cmd"},
						},
					},
					Path: bass.FileOrDirPath{
						File: &bass.FilePath{"file"},
					},
				},
			},
		},
		{
			Enum: &bass.ThunkRunImage{},
			Valid: []bass.Value{
				bass.Bindings{
					"platform": bass.Bindings{
						"os":   bass.String("linux"),
						"arch": bass.String("amd64"),
					}.Scope(),
					"repository": bass.String("repo"),
				}.Scope(),
				bass.Bindings{
					"path": bass.CommandPath{"cmd"},
				}.Scope(),
			},
			Invalid: []bass.Value{
				bass.String("hello"),
				bass.NewEmptyScope(),
				bass.Bindings{
					"bath": bass.CommandPath{"cmd"},
				}.Scope(),
				bass.Null{},
			},
		},
	} {
		test := test

		t.Run(fmt.Sprintf("%T", test.Enum), func(t *testing.T) {
			is := is.New(t)

			for _, v := range test.Valid {
				enum := reflect.New(reflect.TypeOf(test.Enum).Elem()).Interface().(Enum)
				err := enum.FromValue(v)
				is.NoErr(err)
				Equal(t, enum.ToValue(), v)

				payload, err := bass.MarshalJSON(enum)
				is.NoErr(err)

				enum = reflect.New(reflect.TypeOf(test.Enum).Elem()).Interface().(Enum)
				err = enum.UnmarshalJSON(payload)
				is.NoErr(err)
				Equal(t, enum.ToValue(), v)
			}

			for _, v := range test.Invalid {
				enum := reflect.New(reflect.TypeOf(test.Enum).Elem()).Interface().(Enum)
				err := enum.FromValue(v)
				is.True(err != nil)
			}
		})
	}
}
