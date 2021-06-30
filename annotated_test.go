package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestAnnotatedDecode(t *testing.T) {
	val := bass.Annotated{
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

func TestAnnotatedEqual(t *testing.T) {
	val := bass.Annotated{
		Comment: "hello",
		Value: dummyValue{
			sentinel: 42,
		},
	}

	require.True(t, val.Equal(val))
	require.False(t, val.Equal(bass.Annotated{
		Comment: "hello",
		Value: dummyValue{
			sentinel: 43,
		},
	}))

	// compare inner value only
	require.True(t, val.Equal(bass.Annotated{
		Comment: "different",
		Value: dummyValue{
			sentinel: 42,
		},
	}))
}

func TestAnnotatedEval(t *testing.T) {
	env := bass.NewEnv()
	env.Set(bass.Symbol("foo"), bass.Symbol("bar"))

	val := bass.Annotated{
		Comment: "hello",
		Value:   bass.Symbol("foo"),
	}

	res, err := val.Eval(env)
	require.NoError(t, err)
	require.Equal(t, bass.Symbol("bar"), res)

	require.NotEmpty(t, env.Commentary)
	require.ElementsMatch(t, env.Commentary, []bass.Value{
		bass.Annotated{
			Comment: "hello",
			Value:   bass.Symbol("bar"),
		},
	})
	require.Equal(t, env.Docs, bass.Docs{
		"bar": "hello",
	})
}
