package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestScopeDecode(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Def("foo", bass.Int(42))

	var dest *bass.Scope
	err := scope.Decode(&dest)
	require.NoError(t, err)
	require.Equal(t, scope, dest)

	val := bass.Bindings{
		"a": bass.Int(1),
		"b": bass.Bool(true),
		"c": bass.String("three"),
	}.Scope()

	var obj *bass.Scope
	err = val.Decode(&obj)
	require.NoError(t, err)
	require.Equal(t, val, obj)

	var val2 *bass.Scope
	err = val.Decode(&val2)
	require.NoError(t, err)
	require.Equal(t, val, val2)

	type typ struct {
		A int    `json:"a"`
		B bool   `json:"b"`
		C string `json:"c,omitempty"`
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
		A int  `json:"a"`
		B bool `json:"b"`
	}

	var extra extraTyp
	err = val.Decode(&extra)
	require.NoError(t, err)
	require.Equal(t, extraTyp{
		A: 1,
		B: true,
	}, extra)

	type missingTyp struct {
		A int    `json:"a"`
		B bool   `json:"b"`
		C string `json:"c"`
		D string `json:"d"`
	}

	var missing missingTyp
	err = val.Decode(&missing)
	require.Error(t, err)

	type missingOptionalTyp struct {
		A int    `json:"a"`
		B bool   `json:"b"`
		C string `json:"c"`
		D string `json:"d,omitempty"`
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

func TestScopeEqual(t *testing.T) {
	val := bass.NewEmptyScope()
	require.True(t, val.Equal(val))
	require.True(t, val.Equal(bass.NewEmptyScope()))

	scope := bass.Bindings{
		"a": bass.Int(1),
		"b": bass.Bool(true),
	}.Scope()

	wrappedA := bass.Bindings{
		"a": wrappedValue{bass.Int(1)},
		"b": bass.Bool(true),
	}.Scope()

	wrappedB := bass.Bindings{
		"a": bass.Int(1),
		"b": wrappedValue{bass.Bool(true)},
	}.Scope()

	differentA := bass.Bindings{
		"a": bass.Int(2),
		"b": bass.Bool(true),
	}.Scope()

	differentB := bass.Bindings{
		"a": bass.Int(1),
		"b": bass.Bool(false),
	}.Scope()

	missingA := bass.Bindings{
		"b": bass.Bool(true),
	}.Scope()

	require.True(t, scope.Equal(wrappedA))
	require.True(t, scope.Equal(wrappedB))
	require.True(t, wrappedA.Equal(scope))
	require.True(t, wrappedB.Equal(scope))
	require.False(t, scope.Equal(differentA))
	require.False(t, scope.Equal(differentA))
	require.False(t, differentA.Equal(scope))
	require.False(t, differentB.Equal(scope))
	require.False(t, missingA.Equal(scope))
	require.False(t, scope.Equal(missingA))
	require.False(t, val.Equal(bass.Null{}))
}

func TestScopeBinding(t *testing.T) {
	scope := bass.NewEmptyScope()

	val, found := scope.Get(bass.NewSymbol("foo"))
	require.False(t, found)
	require.Nil(t, val)

	scope.Def("foo", bass.Int(42))

	val, found = scope.Get(bass.NewSymbol("foo"))
	require.True(t, found)
	require.Equal(t, bass.Int(42), val)
}

func TestScopeBindingParents(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Def("foo", bass.Int(42))

	child := bass.NewEmptyScope(scope)
	val, found := child.Get(bass.NewSymbol("foo"))
	require.True(t, found)
	require.Equal(t, bass.Int(42), val)
}

func TestScopeBindingParentsOrder(t *testing.T) {
	scope1 := bass.NewEmptyScope()
	scope1.Def("foo", bass.Int(1))

	scope2 := bass.NewEmptyScope()
	scope2.Def("foo", bass.Int(2))
	scope2.Def("bar", bass.Int(3))

	child := bass.NewEmptyScope(scope1, scope2)
	val, found := child.Get(bass.NewSymbol("foo"))
	require.True(t, found)
	require.Equal(t, bass.Int(1), val)

	val, found = child.Get(bass.NewSymbol("bar"))
	require.True(t, found)
	require.Equal(t, bass.Int(3), val)
}

func TestScopeBindingParentsDepthFirst(t *testing.T) {
	scope1Parent := bass.NewEmptyScope()
	scope1Parent.Def("foo", bass.Int(1))

	scope1 := bass.NewEmptyScope(scope1Parent)

	scope2 := bass.NewEmptyScope()
	scope2.Def("foo", bass.Int(2))

	child := bass.NewEmptyScope(scope1, scope2)
	val, found := child.Get(bass.NewSymbol("foo"))
	require.True(t, found)
	require.Equal(t, bass.Int(1), val)
}

func TestScopeBindingDocs(t *testing.T) {
	scope := bass.NewEmptyScope()

	annotated, found := scope.GetWithDoc(bass.NewSymbol("foo"))
	require.False(t, found)
	require.Zero(t, annotated)
	require.Empty(t, scope.Commentary)

	scope.Def("foo", bass.Int(42), "hello", "More info.")

	annotated, found = scope.GetWithDoc(bass.NewSymbol("foo"))
	require.True(t, found)
	require.Equal(t, "hello\n\nMore info.", annotated.Comment)
	require.Equal(t, bass.Int(42), annotated.Value)
	require.NotZero(t, annotated.Range)

	commentary := annotated
	commentary.Value = bass.NewSymbol("foo")
	require.Equal(t, scope.Commentary, []bass.Annotated{commentary})
}

func TestScopeBindingParentsDoc(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Def("foo", bass.Int(42), "hello")

	child := bass.NewEmptyScope(scope)
	annotated, found := child.GetWithDoc(bass.NewSymbol("foo"))
	require.True(t, found)
	require.Equal(t, "hello", annotated.Comment)
	require.Equal(t, bass.Int(42), annotated.Value)
}

func TestScopeBindingParentsOrderDoc(t *testing.T) {
	scope1 := bass.NewEmptyScope()
	scope1.Def("foo", bass.Int(1), "hello 1")

	scope2 := bass.NewEmptyScope()
	scope2.Def("foo", bass.Int(2), "hello 2")
	scope2.Def("bar", bass.Int(3))

	child := bass.NewEmptyScope(scope1, scope2)
	annotated, found := child.GetWithDoc(bass.NewSymbol("foo"))
	require.True(t, found)
	require.Equal(t, "hello 1", annotated.Comment)
	require.Equal(t, bass.Int(1), annotated.Value)

	annotated, found = child.GetWithDoc(bass.NewSymbol("bar"))
	require.True(t, found)
	require.Equal(t, "", annotated.Comment)
	require.Equal(t, bass.Int(3), annotated.Value)
}

func TestScopeBindingParentsDepthFirstDoc(t *testing.T) {
	scope1Parent := bass.NewEmptyScope()
	scope1Parent.Def("foo", bass.Int(1), "hello 1")

	scope1 := bass.NewEmptyScope(scope1Parent)

	scope2 := bass.NewEmptyScope()
	scope2.Def("foo", bass.Int(2), "hello 2")

	child := bass.NewEmptyScope(scope1, scope2)
	annotated, found := child.GetWithDoc(bass.NewSymbol("foo"))
	require.True(t, found)
	require.Equal(t, "hello 1", annotated.Comment)
	require.Equal(t, bass.Int(1), annotated.Value)
}
