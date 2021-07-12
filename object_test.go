package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestObjectDecode(t *testing.T) {
	val := bass.Object{
		"a": bass.Int(1),
		"b": bass.Bool(true),
		"c": bass.String("three"),
	}

	var obj bass.Object
	err := val.Decode(&obj)
	require.NoError(t, err)
	require.Equal(t, val, obj)

	type typ struct {
		A int    `bass:"a"`
		B bool   `bass:"b"`
		C string `bass:"c"`
	}

	var native typ
	err = val.Decode(&native)
	require.NoError(t, err)
	require.Equal(t, typ{
		A: 1,
		B: true,
		C: "three",
	}, native)

	type extraTyp struct {
		A int  `bass:"a"`
		B bool `bass:"b"`
	}

	var extra extraTyp
	err = val.Decode(&extra)
	require.NoError(t, err)
	require.Equal(t, extraTyp{
		A: 1,
		B: true,
	}, extra)

	type missingTyp struct {
		A int    `bass:"a"`
		B bool   `bass:"b"`
		C string `bass:"c"`
		D string `bass:"d"`
	}

	var missing missingTyp
	err = val.Decode(&missing)
	require.Error(t, err)

	type missingOptionalTyp struct {
		A int    `bass:"a"`
		B bool   `bass:"b"`
		C string `bass:"c"`
		D string `bass:"d" optional:"true"`
	}

	var missingOptional missingOptionalTyp
	err = val.Decode(&missingOptional)
	require.NoError(t, err)
	require.Equal(t, missingOptionalTyp{
		A: 1,
		B: true,
		C: "three",
		D: "",
	}, missingOptional)
}

func TestObjectEqual(t *testing.T) {
	obj := bass.Object{
		"a": bass.Int(1),
		"b": bass.Bool(true),
	}

	wrappedA := bass.Object{
		"a": wrappedValue{bass.Int(1)},
		"b": bass.Bool(true),
	}

	wrappedB := bass.Object{
		"a": bass.Int(1),
		"b": wrappedValue{bass.Bool(true)},
	}

	differentA := bass.Object{
		"a": bass.Int(2),
		"b": bass.Bool(true),
	}

	differentB := bass.Object{
		"a": bass.Int(1),
		"b": bass.Bool(false),
	}

	missingA := bass.Object{
		"b": bass.Bool(true),
	}

	val := bass.NewEnv()
	require.True(t, obj.Equal(wrappedA))
	require.True(t, obj.Equal(wrappedB))
	require.True(t, wrappedA.Equal(obj))
	require.True(t, wrappedB.Equal(obj))
	require.False(t, obj.Equal(differentA))
	require.False(t, obj.Equal(differentA))
	require.False(t, differentA.Equal(obj))
	require.False(t, differentB.Equal(obj))
	require.False(t, missingA.Equal(obj))
	require.False(t, obj.Equal(missingA))
	require.False(t, val.Equal(bass.Null{}))
}
