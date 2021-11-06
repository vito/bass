package bass_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/vito/bass"
	"github.com/vito/is"
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
		bass.NewEmptyScope(),
		bass.Bindings{
			"a": bass.Bool(true),
			"b": bass.Int(1),
			"c": bass.String("hello"),
		}.Scope(),
		bass.Bindings{
			"hyphenated-key": bass.String("hello"),
		}.Scope(),
		bass.NewList(
			bass.Bool(true),
			bass.Int(1),
			bass.Bindings{
				"a": bass.Bool(true),
				"b": bass.Int(1),
				"c": bass.String("hello"),
			}.Scope(),
		),
		bass.Bindings{
			"a": bass.Bool(true),
			"b": bass.Int(1),
			"c": bass.NewList(
				bass.Bool(true),
				bass.Int(1),
				bass.String("hello"),
			),
		}.Scope(),
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
		bass.Bind{
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
			is := is.New(t)

			_, err := bass.MarshalJSON(val)
			is.True(err != nil)

			var marshalErr bass.EncodeError
			is.True(errors.As(err, &marshalErr))
			is.Equal(marshalErr.Value, val)
		})
	}
}

func testJSONValueDecodeLifecycle(t *testing.T, val interface{}) {
	is := is.New(t)

	type_ := reflect.TypeOf(val)

	payload, err := bass.MarshalJSON(val)
	is.NoErr(err)

	t.Logf("value -> json: %s", string(payload))

	dest := reflect.New(type_)
	err = bass.UnmarshalJSON(payload, dest.Interface())
	is.NoErr(err)

	t.Logf("json -> value: %+v", dest.Interface())

	is.True(val.(bass.Value).Equal(dest.Elem().Interface().(bass.Value)))

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
	is.NoErr(err)

	t.Logf("struct -> json: %s", string(payload))

	dest = reflect.New(structType)
	err = bass.UnmarshalJSON(payload, dest.Interface())
	is.NoErr(err)

	t.Logf("json -> struct: %+v", dest.Interface())

	is.Equal(dest.Interface(), object.Interface())

	var iface interface{}
	err = bass.UnmarshalJSON(payload, &iface)
	is.NoErr(err)

	t.Logf("json -> iface: %s", iface)

	ifaceVal, err := bass.ValueOf(iface)
	is.NoErr(err)

	t.Logf("iface -> value: %s", ifaceVal)

	objDest := ifaceVal.(*bass.Scope)

	dest = reflect.New(structType)
	err = objDest.Decode(dest.Interface())
	is.NoErr(err)

	t.Logf("value -> struct: %+v", dest.Interface())

	is.Equal(dest.Interface(), object.Interface())
}
