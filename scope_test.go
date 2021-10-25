package bass_test

import (
	"testing"

	"github.com/vito/bass"
	"github.com/vito/is"
)

func TestScopeDecode(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	scope.Set("foo", bass.Int(42))

	var dest *bass.Scope
	err := scope.Decode(&dest)
	is.NoErr(err)
	is.Equal(dest, scope)

	val := bass.Bindings{
		"a": bass.Int(1),
		"b": bass.Bool(true),
		"c": bass.String("three"),
	}.Scope()

	var obj *bass.Scope
	err = val.Decode(&obj)
	is.NoErr(err)
	is.Equal(obj, val)

	var val2 *bass.Scope
	err = val.Decode(&val2)
	is.NoErr(err)
	is.Equal(val2, val)

	type typ struct {
		A int    `json:"a"`
		B bool   `json:"b"`
		C string `json:"c,omitempty"`
	}

	var native typ
	err = val.Decode(&native)
	is.NoErr(err)
	is.Equal(

		native, typ{
			A: 1,
			B: true,
			C: "three",
		})

	type extraTyp struct {
		A int  `json:"a"`
		B bool `json:"b"`
	}

	var extra extraTyp
	err = val.Decode(&extra)
	is.NoErr(err)
	is.Equal(

		extra, extraTyp{
			A: 1,
			B: true,
		})

	type missingTyp struct {
		A int    `json:"a"`
		B bool   `json:"b"`
		C string `json:"c"`
		D string `json:"d"`
	}

	var missing missingTyp
	err = val.Decode(&missing)
	is.True(err != nil)

	type missingOptionalTyp struct {
		A int    `json:"a"`
		B bool   `json:"b"`
		C string `json:"c"`
		D string `json:"d,omitempty"`
	}

	var missingOptional missingOptionalTyp
	err = val.Decode(&missingOptional)
	is.NoErr(err)
	is.Equal(

		missingOptional, missingOptionalTyp{
			A: 1,
			B: true,
			C: "three",
			D: "",
		})

}

func TestScopeEqual(t *testing.T) {
	is := is.New(t)

	val := bass.NewEmptyScope()
	is.True(val.Equal(val))
	is.True(val.Equal(bass.NewEmptyScope()))

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

	is.True(scope.Equal(wrappedA))
	is.True(scope.Equal(wrappedB))
	is.True(wrappedA.Equal(scope))
	is.True(wrappedB.Equal(scope))
	is.True(!scope.Equal(differentA))
	is.True(!scope.Equal(differentA))
	is.True(!differentA.Equal(scope))
	is.True(!differentB.Equal(scope))
	is.True(!missingA.Equal(scope))
	is.True(!scope.Equal(missingA))
	is.True(!val.Equal(bass.Null{}))
}

func TestScopeBinding(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()

	val, found := scope.Get("foo")
	is.True(!found)
	is.True(val == nil)

	scope.Set("foo", bass.Int(42))

	val, found = scope.Get("foo")
	is.True(found)
	is.Equal(val, bass.Int(42))
}

func TestScopeBindingParents(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	scope.Set("foo", bass.Int(42))

	child := bass.NewEmptyScope(scope)
	val, found := child.Get("foo")
	is.True(found)
	is.Equal(val, bass.Int(42))
}

func TestScopeBindingParentsOrder(t *testing.T) {
	is := is.New(t)

	scope1 := bass.NewEmptyScope()
	scope1.Set("foo", bass.Int(1))

	scope2 := bass.NewEmptyScope()
	scope2.Set("foo", bass.Int(2))
	scope2.Set("bar", bass.Int(3))

	child := bass.NewEmptyScope(scope1, scope2)
	val, found := child.Get("foo")
	is.True(found)
	is.Equal(val, bass.Int(1))

	val, found = child.Get("bar")
	is.True(found)
	is.Equal(val, bass.Int(3))
}

func TestScopeBindingParentsDepthFirst(t *testing.T) {
	is := is.New(t)

	scope1Parent := bass.NewEmptyScope()
	scope1Parent.Set("foo", bass.Int(1))

	scope1 := bass.NewEmptyScope(scope1Parent)

	scope2 := bass.NewEmptyScope()
	scope2.Set("foo", bass.Int(2))

	child := bass.NewEmptyScope(scope1, scope2)
	val, found := child.Get("foo")
	is.True(found)
	is.Equal(val, bass.Int(1))
}

// func TestScopeBindingDocs(t *testing.T) {
// 	is := is.New(t)

// 	scope := bass.NewEmptyScope()
// 	is.True(len(scope.Commentary) == 0)

// 	_, found := scope.GetWithDoc("foo")
// 	is.True(!found)

// 	scope.Set("foo", bass.Int(42), "hello", "More info.")

// 	annotated, found := scope.GetWithDoc("foo")
// 	is.True(found)
// 	is.Equal(annotated.Comment, "hello\n\nMore info.")
// 	is.Equal(annotated.Value, bass.Int(42))
// 	s.True(strings.HasSuffix(annotated.Range.Start.File, "scope_test.go"))
// 	s.True(annotated.Range.Start.Ln > 0)

// 	commentary := annotated
// 	commentary.Value = bass.Symbol("foo")
// 	is.Equal([]bass.Annotated{commentary}, scope.Commentary)
// }

// func TestScopeBindingParentsDoc(t *testing.T) {
// 	is := is.New(t)

// 	scope := bass.NewEmptyScope()
// 	scope.Set("foo", bass.Int(42), "hello")

// 	child := bass.NewEmptyScope(scope)
// 	annotated, found := child.GetWithDoc("foo")
// 	is.True(found)
// 	is.Equal(annotated.Comment, "hello")
// 	is.Equal(annotated.Value, bass.Int(42))
// }

// func TestScopeBindingParentsOrderDoc(t *testing.T) {
// 	is := is.New(t)

// 	scope1 := bass.NewEmptyScope()
// 	scope1.Set("foo", bass.Int(1), "hello 1")

// 	scope2 := bass.NewEmptyScope()
// 	scope2.Set("foo", bass.Int(2), "hello 2")
// 	scope2.Set("bar", bass.Int(3))

// 	child := bass.NewEmptyScope(scope1, scope2)
// 	annotated, found := child.GetWithDoc("foo")
// 	is.True(found)
// 	is.Equal(annotated.Comment, "hello 1")
// 	is.Equal(annotated.Value, bass.Int(1))

// 	annotated, found = child.GetWithDoc("bar")
// 	is.True(found)
// 	is.Equal(annotated.Comment, "")
// 	is.Equal(annotated.Value, bass.Int(3))
// }

// func TestScopeBindingParentsDepthFirstDoc(t *testing.T) {
// 	is := is.New(t)

// 	scope1Parent := bass.NewEmptyScope()
// 	scope1Parent.Set("foo", bass.Int(1), "hello 1")

// 	scope1 := bass.NewEmptyScope(scope1Parent)

// 	scope2 := bass.NewEmptyScope()
// 	scope2.Set("foo", bass.Int(2), "hello 2")

// 	child := bass.NewEmptyScope(scope1, scope2)
// 	annotated, found := child.GetWithDoc("foo")
// 	is.True(found)
// 	is.Equal(annotated.Comment, "hello 1")
// 	is.Equal(annotated.Value, bass.Int(1))
// }
