package bass_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestPrelude(t *testing.T) {
	env := bass.New()

	type example struct {
		Name   string
		Bass   string
		Result bass.Value
	}

	for _, test := range []example{
		{
			Name:   "+",
			Bass:   "(+ 1 2 3)",
			Result: bass.Int(6),
		},
	} {
		reader := bass.NewReader(bytes.NewBufferString(test.Bass))

		val, err := reader.Next()
		require.NoError(t, err)

		res, err := val.Eval(env)
		require.NoError(t, err)

		require.Equal(t, test.Result, res)
	}
}
