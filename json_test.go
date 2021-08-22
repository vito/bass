package bass_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
)

func TestJSONable(t *testing.T) {
	for _, val := range []interface{}{
		bass.Null{},
		bass.Empty{},
		bass.Bool(true),
		bass.Bool(false),
		bass.Int(42),
		bass.NewList(
			bass.Bool(true),
			bass.Int(1),
			bass.String("hello"),
		),
		bass.Object{
			"a": bass.Bool(true),
			"b": bass.Int(1),
			"c": bass.String("hello"),
		},
		bass.Object{
			"hyphenated-key": bass.String("hello"),
		},
		bass.NewList(
			bass.Bool(true),
			bass.Int(1),
			bass.Object{
				"a": bass.Bool(true),
				"b": bass.Int(1),
				"c": bass.String("hello"),
			},
		),
		bass.Object{
			"a": bass.Bool(true),
			"b": bass.Int(1),
			"c": bass.NewList(
				bass.Bool(true),
				bass.Int(1),
				bass.String("hello"),
			),
		},
		bass.DirPath{"directory-path"},
		bass.FilePath{"file-path"},
		bass.CommandPath{"command-path"},
	} {
		val := val
		t.Run(fmt.Sprintf("%T", val), func(t *testing.T) {
			testJSONValueDecodeLifecycle(t, val)
		})
	}
}

func TestUnJSONable(t *testing.T) {
	for _, val := range []bass.Value{
		bass.Op("noop", "[]", func() {}),
		bass.Func("nofn", "[]", func() {}),
		bass.NewEnv(),
		operative,
		bass.Wrapped{operative},
		bass.Stdin,
		bass.Stdout,
		&bass.Continuation{
			Continue: func(x bass.Value) bass.Value {
				return x
			},
		},
		&bass.ReadyContinuation{
			Continuation: &bass.Continuation{
				Continue: func(x bass.Value) bass.Value {
					return x
				},
			},
			Result: bass.Int(42),
		},
		bass.Pair{
			A: bass.String("a"),
			D: bass.String("d"),
		},
		bass.Cons{
			A: bass.String("a"),
			D: bass.String("d"),
		},
		bass.Assoc{
			bass.Pair{
				A: bass.String("a"),
				D: bass.String("d"),
			},
		},
		bass.Annotated{
			Value:   bass.String("foo"),
			Comment: "annotated",
		},
	} {
		val := val
		t.Run(fmt.Sprintf("%T", val), func(t *testing.T) {
			_, err := bass.MarshalJSON(val)
			require.Error(t, err)

			var marshalErr bass.EncodeError
			require.ErrorAs(t, err, &marshalErr)
			require.Equal(t, val, marshalErr.Value)
		})
	}
}

func testJSONValueDecodeLifecycle(t *testing.T, val interface{}) {
	type_ := reflect.TypeOf(val)

	payload, err := bass.MarshalJSON(val)
	require.NoError(t, err)

	t.Logf("value -> json: %s", string(payload))

	dest := reflect.New(type_)
	err = bass.UnmarshalJSON(payload, dest.Interface())
	require.NoError(t, err)

	t.Logf("json -> value: %+v", dest.Interface())

	Equal(t, val.(bass.Value), dest.Elem().Interface().(bass.Value))

	structType := reflect.StructOf([]reflect.StructField{
		{
			Name: "Value",
			Type: reflect.TypeOf(val),
			Tag:  `json:"value"`,
		},
	})

	object := reflect.New(structType)
	object.Elem().Field(0).Set(reflect.ValueOf(val))

	t.Logf("value -> struct: %+v", object.Interface())

	payload, err = bass.MarshalJSON(object.Interface())
	require.NoError(t, err)

	t.Logf("struct -> json: %s", string(payload))

	dest = reflect.New(structType)
	err = bass.UnmarshalJSON(payload, dest.Interface())
	require.NoError(t, err)

	t.Logf("json -> struct: %+v", dest.Interface())

	require.Equal(t, object.Interface(), dest.Interface())

	var iface interface{}
	err = bass.UnmarshalJSON(payload, &iface)
	require.NoError(t, err)

	t.Logf("json -> iface: %s", iface)

	ifaceVal, err := bass.ValueOf(iface)
	require.NoError(t, err)

	t.Logf("iface -> value: %s", ifaceVal)

	objDest := ifaceVal.(bass.Object)

	dest = reflect.New(structType)
	err = objDest.Decode(dest.Interface())
	require.NoError(t, err)

	t.Logf("value -> struct: %+v", dest.Interface())

	require.Equal(t, object.Interface(), dest.Interface())
}
