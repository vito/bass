package bass_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

type Enum interface {
	FromValue(bass.Value) error
	ToValue() bass.Value

	json.Marshaler
	json.Unmarshaler
}

func TestEnums(t *testing.T) {
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
				bass.Object{
					"repository": bass.String("repo"),
				},
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
				bass.Object{},
				bass.Null{},
			},
		},
	} {
		test := test

		t.Run(fmt.Sprintf("%T", test.Enum), func(t *testing.T) {
			for _, v := range test.Valid {
				enum := reflect.New(reflect.TypeOf(test.Enum).Elem()).Interface().(Enum)
				err := enum.FromValue(v)
				require.NoError(t, err)
				require.Equal(t, v, enum.ToValue())

				payload, err := bass.MarshalJSON(enum)
				require.NoError(t, err)

				enum = reflect.New(reflect.TypeOf(test.Enum).Elem()).Interface().(Enum)
				err = enum.UnmarshalJSON(payload)
				require.NoError(t, err)
				require.Equal(t, v, enum.ToValue())
			}

			for _, v := range test.Invalid {
				enum := reflect.New(reflect.TypeOf(test.Enum).Elem()).Interface().(Enum)
				err := enum.FromValue(v)
				require.Error(t, err)
			}
		})
	}
}
