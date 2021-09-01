package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestScopeDecode(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Set("foo", bass.Int(42))

	var dest *bass.Scope
	err := scope.Decode(&dest)
	require.NoError(t, err)
	require.Equal(t, scope, dest)
}

func TestScopeEqual(t *testing.T) {
	val := bass.NewEmptyScope()
	require.True(t, val.Equal(val))
	require.False(t, val.Equal(bass.NewEmptyScope()))
}

func TestScopeBinding(t *testing.T) {
	scope := bass.NewEmptyScope()

	val, found := scope.Get("foo")
	require.False(t, found)
	require.Nil(t, val)

	scope.Set("foo", bass.Int(42))

	val, found = scope.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(42), val)
}

func TestScopeBindingParents(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Set("foo", bass.Int(42))

	child := bass.NewEmptyScope(scope)
	val, found := child.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(42), val)
}

func TestScopeBindingParentsOrder(t *testing.T) {
	scope1 := bass.NewEmptyScope()
	scope1.Set("foo", bass.Int(1))

	scope2 := bass.NewEmptyScope()
	scope2.Set("foo", bass.Int(2))
	scope2.Set("bar", bass.Int(3))

	child := bass.NewEmptyScope(scope1, scope2)
	val, found := child.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(1), val)

	val, found = child.Get("bar")
	require.True(t, found)
	require.Equal(t, bass.Int(3), val)
}

func TestScopeBindingParentsDepthFirst(t *testing.T) {
	scope1Parent := bass.NewEmptyScope()
	scope1Parent.Set("foo", bass.Int(1))

	scope1 := bass.NewEmptyScope(scope1Parent)

	scope2 := bass.NewEmptyScope()
	scope2.Set("foo", bass.Int(2))

	child := bass.NewEmptyScope(scope1, scope2)
	val, found := child.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(1), val)
}

func TestScopeBindingDocs(t *testing.T) {
	scope := bass.NewEmptyScope()

	annotated, found := scope.GetWithDoc("foo")
	require.False(t, found)
	require.Zero(t, annotated)
	require.Empty(t, scope.Commentary)

	scope.Set("foo", bass.Int(42), "hello", "More info.")

	annotated, found = scope.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello\n\nMore info.", annotated.Comment)
	require.Equal(t, bass.Int(42), annotated.Value)
	require.NotZero(t, annotated.Range)

	commentary := annotated
	commentary.Value = bass.Keyword("foo")
	require.Equal(t, scope.Commentary, []bass.Annotated{commentary})
}

func TestScopeBindingParentsDoc(t *testing.T) {
	scope := bass.NewEmptyScope()
	scope.Set("foo", bass.Int(42), "hello")

	child := bass.NewEmptyScope(scope)
	annotated, found := child.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello", annotated.Comment)
	require.Equal(t, bass.Int(42), annotated.Value)
}

func TestScopeBindingParentsOrderDoc(t *testing.T) {
	scope1 := bass.NewEmptyScope()
	scope1.Set("foo", bass.Int(1), "hello 1")

	scope2 := bass.NewEmptyScope()
	scope2.Set("foo", bass.Int(2), "hello 2")
	scope2.Set("bar", bass.Int(3))

	child := bass.NewEmptyScope(scope1, scope2)
	annotated, found := child.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello 1", annotated.Comment)
	require.Equal(t, bass.Int(1), annotated.Value)

	annotated, found = child.GetWithDoc("bar")
	require.True(t, found)
	require.Equal(t, "", annotated.Comment)
	require.Equal(t, bass.Int(3), annotated.Value)
}

func TestScopeBindingParentsDepthFirstDoc(t *testing.T) {
	scope1Parent := bass.NewEmptyScope()
	scope1Parent.Set("foo", bass.Int(1), "hello 1")

	scope1 := bass.NewEmptyScope(scope1Parent)

	scope2 := bass.NewEmptyScope()
	scope2.Set("foo", bass.Int(2), "hello 2")

	child := bass.NewEmptyScope(scope1, scope2)
	annotated, found := child.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello 1", annotated.Comment)
	require.Equal(t, bass.Int(1), annotated.Value)
}
