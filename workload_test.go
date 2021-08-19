package bass_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestWorkloadName(t *testing.T) {
	// use an object with a ton of keys to test stable order when hashing
	manyKeys := bass.Object{}
	for i := 0; i < 100; i++ {
		manyKeys[bass.Keyword(fmt.Sprintf("key-%d", i))] = bass.Int(i)
	}

	workload := bass.Workload{
		Platform: manyKeys,
		Path: bass.RunPath{
			File: &bass.FilePath{"run"},
		},
		Env: manyKeys,
	}

	name, err := workload.SHA1()
	require.NoError(t, err)

	// this is a bit silly, but it's deterministic, and we need to make sure it's
	// always the same value
	require.Equal(t, "78f4240582a43dc25aa9f92ca0fb5edb5cf07e06", name)
}
