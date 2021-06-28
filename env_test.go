package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestEnvDecode(t *testing.T) {
	env := bass.NewEnv()
	env.Set("foo", bass.Int(42))

	var dest *bass.Env
	err := env.Decode(&dest)
	require.NoError(t, err)
	require.Equal(t, env, dest)
}

func TestEnvEval(t *testing.T) {
	env := bass.NewEnv()
	val := bass.NewEnv()
	val.Set("foo", bass.Int(42)) // just to strengthen Equal check

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, val, res)
}

func TestEnvBinding(t *testing.T) {
	env := bass.NewEnv()

	val, found := env.Get("foo")
	require.False(t, found)
	require.Nil(t, val)

	env.Set("foo", bass.Int(42))

	val, found = env.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(42), val)
}

func TestEnvBindingDocs(t *testing.T) {
	env := bass.NewEnv()

	val, doc, found := env.GetWithDoc("foo")
	require.False(t, found)
	require.Empty(t, doc)
	require.Nil(t, val)

	env.Set("foo", bass.Int(42), "hello", "More info.")

	val, doc, found = env.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello\n\nMore info.", doc)
	require.Equal(t, bass.Int(42), val)
}

func TestEnvBindingParents(t *testing.T) {
	env := bass.NewEnv()
	env.Set("foo", bass.Int(42))

	child := bass.NewEnv(env)
	val, found := child.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(42), val)
}

func TestEnvBindingParentsOrder(t *testing.T) {
	env1 := bass.NewEnv()
	env1.Set("foo", bass.Int(1))

	env2 := bass.NewEnv()
	env2.Set("foo", bass.Int(2))
	env2.Set("bar", bass.Int(3))

	child := bass.NewEnv(env1, env2)
	val, found := child.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(1), val)

	val, found = child.Get("bar")
	require.True(t, found)
	require.Equal(t, bass.Int(3), val)
}

func TestEnvBindingParentsDepthFirst(t *testing.T) {
	env1Parent := bass.NewEnv()
	env1Parent.Set("foo", bass.Int(1))

	env1 := bass.NewEnv(env1Parent)

	env2 := bass.NewEnv()
	env2.Set("foo", bass.Int(2))

	child := bass.NewEnv(env1, env2)
	val, found := child.Get("foo")
	require.True(t, found)
	require.Equal(t, bass.Int(1), val)
}

func TestEnvBindingDoc(t *testing.T) {
	env := bass.NewEnv()

	val, doc, found := env.GetWithDoc("foo")
	require.False(t, found)
	require.Empty(t, doc)
	require.Nil(t, val)

	env.Set("foo", bass.Int(42), "hello")

	val, doc, found = env.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello", doc)
	require.Equal(t, bass.Int(42), val)
}

func TestEnvBindingParentsDoc(t *testing.T) {
	env := bass.NewEnv()
	env.Set("foo", bass.Int(42), "hello")

	child := bass.NewEnv(env)
	val, doc, found := child.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello", doc)
	require.Equal(t, bass.Int(42), val)
}

func TestEnvBindingParentsOrderDoc(t *testing.T) {
	env1 := bass.NewEnv()
	env1.Set("foo", bass.Int(1), "hello 1")

	env2 := bass.NewEnv()
	env2.Set("foo", bass.Int(2), "hello 2")
	env2.Set("bar", bass.Int(3))

	child := bass.NewEnv(env1, env2)
	val, doc, found := child.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello 1", doc)
	require.Equal(t, bass.Int(1), val)

	val, doc, found = child.GetWithDoc("bar")
	require.True(t, found)
	require.Equal(t, "", doc)
	require.Equal(t, bass.Int(3), val)
}

func TestEnvBindingParentsDepthFirstDoc(t *testing.T) {
	env1Parent := bass.NewEnv()
	env1Parent.Set("foo", bass.Int(1), "hello 1")

	env1 := bass.NewEnv(env1Parent)

	env2 := bass.NewEnv()
	env2.Set("foo", bass.Int(2), "hello 2")

	child := bass.NewEnv(env1, env2)
	val, doc, found := child.GetWithDoc("foo")
	require.True(t, found)
	require.Equal(t, "hello 1", doc)
	require.Equal(t, bass.Int(1), val)
}

func TestEnvDefine(t *testing.T) {
	type example struct {
		Name   string
		Params bass.Value
		Value  bass.Value

		Bindings bass.Bindings
		Err      error
	}

	for _, test := range []example{
		{
			Name:   "symbol",
			Params: bass.Symbol("foo"),
			Value:  bass.String("hello"),
			Bindings: bass.Bindings{
				"foo": bass.String("hello"),
			},
		},
		{
			Name:   "list",
			Params: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
			Value:  bass.NewList(bass.Int(1), bass.Int(2)),
			Bindings: bass.Bindings{
				"a": bass.Int(1),
				"b": bass.Int(2),
			},
		},
		{
			Name:     "empty ok with empty",
			Params:   bass.Empty{},
			Value:    bass.Empty{},
			Bindings: bass.Bindings{},
		},
		{
			Name:   "empty err on extra",
			Params: bass.Empty{},
			Value:  bass.NewList(bass.Int(1), bass.Int(2)),
			Err: bass.BindMismatchError{
				Need: bass.Empty{},
				Have: bass.NewList(bass.Int(1), bass.Int(2)),
			},
		},
		{
			Name:   "list err with empty",
			Params: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
			Value:  bass.Empty{},
			Err: bass.BindMismatchError{
				Need: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
				Have: bass.Empty{},
			},
		},
		{
			Name:   "list err with missing value",
			Params: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
			Value:  bass.NewList(bass.Int(1)),
			Err: bass.BindMismatchError{
				Need: bass.NewList(bass.Symbol("b")),
				Have: bass.Empty{},
			},
		},
		{
			Name: "pair",
			Params: bass.Pair{
				A: bass.Symbol("a"),
				D: bass.Symbol("d"),
			},
			Value: bass.NewList(bass.Int(1), bass.Int(2)),
			Bindings: bass.Bindings{
				"a": bass.Int(1),
				"d": bass.NewList(bass.Int(2)),
			},
		},
		{
			Name:   "list with pair",
			Params: bass.NewList(bass.Symbol("a"), bass.Symbol("b")),
			Value: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
			Err: bass.BindMismatchError{
				Need: bass.NewList(bass.Symbol("b")),
				Have: bass.Int(2),
			},
		},
		{
			Name:   "unassignable",
			Params: bass.NewList(bass.Int(1)),
			Value: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
			Err: bass.CannotBindError{
				Have: bass.Int(1),
			},
		},
		{
			Name:   "ignore",
			Params: bass.Ignore{},
			Value: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
			Bindings: bass.Bindings{},
		},
		{
			Name: "bind and ignore",
			Params: bass.Pair{
				A: bass.Ignore{},
				D: bass.Symbol("b"),
			},
			Value: bass.Pair{
				A: bass.Int(1),
				D: bass.Int(2),
			},
			Bindings: bass.Bindings{
				"b": bass.Int(2),
			},
		},
		{
			Name:   "binding ignore",
			Params: bass.Symbol("i"),
			Value:  bass.Ignore{},
			Bindings: bass.Bindings{
				"i": bass.Ignore{},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			env := bass.NewEnv()

			err := env.Define(test.Params, test.Value)
			if test.Err != nil {
				require.Equal(t, test.Err, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.Bindings, env.Bindings)
			}
		})
	}
}
