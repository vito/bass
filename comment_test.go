package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestCommentedDecode(t *testing.T) {
	val := bass.Commented{
		Comment: "hello",
		Value: dummyValue{
			sentinel: 42,
		},
	}

	var dest dummyValue
	err := val.Decode(&dest)
	require.NoError(t, err)
	require.Equal(t, val.Value, dest)
}

func TestCommentedEval(t *testing.T) {
	env := bass.NewEnv()
	env.Set(bass.Symbol("foo"), bass.Symbol("bar"))

	val := bass.Commented{
		Comment: "hello",
		Value:   bass.Symbol("foo"),
	}

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, bass.Symbol("bar"), res)

	require.NotEmpty(t, env.Commentary)
	require.ElementsMatch(t, env.Commentary, []bass.Value{
		bass.Commented{
			Comment: "hello",
			Value:   bass.Symbol("bar"),
		},
	})
	require.Equal(t, env.Docs, bass.Docs{
		"bar": "hello",
	})
}
