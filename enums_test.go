package bass_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
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
			Enum: &bass.MountSourceEnum{},
			Valid: []bass.Value{
				bass.DirPath{"dir"},
				bass.FilePath{"file"},
				bass.WorkloadPath{
					Workload: bass.Workload{
						Path: bass.RunPath{
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
			},
		},
		{
			Enum: &bass.RunPath{},
			Valid: []bass.Value{
				bass.CommandPath{"cmd"},
				bass.FilePath{"file"},
				bass.WorkloadPath{
					Workload: bass.Workload{
						Path: bass.RunPath{
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
				bass.WorkloadPath{
					Workload: bass.Workload{
						Path: bass.RunPath{
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
			Enum: &bass.RunDirPath{},
			Valid: []bass.Value{
				bass.DirPath{"dir"},
				bass.WorkloadPath{
					Workload: bass.Workload{
						Path: bass.RunPath{
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
				bass.WorkloadPath{
					Workload: bass.Workload{
						Path: bass.RunPath{
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
			Enum: &bass.ImageEnum{},
			Valid: []bass.Value{
				bass.Bindings{
					"repository": bass.String("repo")}.Scope(),

				bass.WorkloadPath{
					Workload: bass.Workload{
						Path: bass.RunPath{
							Cmd: &bass.CommandPath{"cmd"},
						},
					},
					Path: bass.FileOrDirPath{
						File: &bass.FilePath{"file"},
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
				enum := reflect.New(reflect.TypeOf(test.Enum).Elem()).Interface().(Enum)
				err := enum.FromValue(v)
				is.NoErr(err)
				is.Equal(enum.ToValue(), v)

				payload, err := bass.MarshalJSON(enum)
				is.NoErr(err)

				enum = reflect.New(reflect.TypeOf(test.Enum).Elem()).Interface().(Enum)
				err = enum.UnmarshalJSON(payload)
				is.NoErr(err)
				is.Equal(enum.ToValue(), v)
			}

			for _, v := range test.Invalid {
				enum := reflect.New(reflect.TypeOf(test.Enum).Elem()).Interface().(Enum)
				err := enum.FromValue(v)
				is.True(err != nil)
			}
		})
	}
}
