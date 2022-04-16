package bass_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
)

func TestJSONable(t *testing.T) {
	for _, val := range []bass.Value{
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
		bass.NewHostPath("./", bass.ParseFileOrDirPath("foo")),
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
			Cont: &bass.Continuation{
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
		bass.Annotate{
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

func testJSONValueDecodeLifecycle(t *testing.T, val bass.Value) {
	t.Run("basic marshaling", func(t *testing.T) {
		is := is.New(t)

		type_ := reflect.TypeOf(val)

		payload, err := bass.MarshalJSON(val)
		is.NoErr(err)

		t.Logf("value -> json: %s", string(payload))

		dest := reflect.New(type_)
		err = bass.UnmarshalJSON(payload, dest.Interface())
		is.NoErr(err)

		t.Logf("json -> value: %+v", dest.Interface())

		equalSameType(t, val, dest.Elem().Interface().(bass.Value))
	})

	t.Run("in a list", func(t *testing.T) {
		is := is.New(t)

		payload, err := bass.MarshalJSON(bass.NewList(val))
		is.NoErr(err)

		t.Logf("value -> list -> json: %s", string(payload))

		var vals []bass.Value
		err = bass.UnmarshalJSON(payload, &vals)
		is.NoErr(err)

		t.Logf("json -> list -> value: %+v", vals[0])

		equalSameType(t, val, vals[0])
	})

	t.Run("in a scope", func(t *testing.T) {
		is := is.New(t)

		payload, err := bass.MarshalJSON(bass.Bindings{"foo": val}.Scope())
		is.NoErr(err)

		t.Logf("value -> scope -> json: %s", string(payload))

		var scp *bass.Scope
		err = bass.UnmarshalJSON(payload, &scp)
		is.NoErr(err)

		v, found := scp.Get("foo")
		is.True(found)
		t.Logf("json -> scope -> value: %+v", v)

		equalSameType(t, val, v)
	})

	t.Run("in a thunk", func(t *testing.T) {
		is := is.New(t)

		payload, err := bass.MarshalJSON(bass.MustThunk(bass.CommandPath{"foo"}, val))
		is.NoErr(err)

		t.Logf("value -> list -> json: %s", string(payload))

		var thunk bass.Thunk
		err = bass.UnmarshalJSON(payload, &thunk)
		is.NoErr(err)

		t.Logf("json -> list -> value: %+v", thunk.Stdin[0])

		equalSameType(t, val, thunk.Stdin[0])
	})

	t.Run("in a struct", func(t *testing.T) {
		is := is.New(t)

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

		payload, err := bass.MarshalJSON(object.Interface())
		is.NoErr(err)

		t.Logf("struct -> json: %s", string(payload))

		dest := reflect.New(structType)
		err = bass.UnmarshalJSON(payload, dest.Interface())
		is.NoErr(err)

		t.Logf("json -> struct: %+v", dest.Interface())

		equalSameType(t, val, dest.Elem().Field(0).Interface().(bass.Value))

		var ifaceVal bass.Value
		err = bass.UnmarshalJSON(payload, &ifaceVal)
		is.NoErr(err)

		t.Logf("json -> value: %s", ifaceVal)

		objDest := ifaceVal.(*bass.Scope)

		dest = reflect.New(structType)
		err = objDest.Decode(dest.Interface())
		is.NoErr(err)

		t.Logf("value -> struct: %+v", dest.Interface())

		equalSameType(t, val, dest.Elem().Field(0).Interface().(bass.Value))
	})
}

func equalSameType(t *testing.T, expected, actual bass.Value) {
	t.Helper()
	is.New(t).Equal(reflect.TypeOf(expected), reflect.TypeOf(actual))
	Equal(t, expected, actual)
}
