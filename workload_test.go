package bass_test

import (
	"fmt"
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
)

func TestWorkloadName(t *testing.T) {
	is := is.New(t)

	// use an object with a ton of keys to test stable order when hashing
	manyKeys := bass.NewEmptyScope()
	for i := 0; i < 100; i++ {
		manyKeys.Set(bass.Symbol(fmt.Sprintf("key-%d", i)), bass.Int(i))
	}

	workload := bass.Workload{
		Platform: manyKeys,
		Path: bass.RunPath{
			File: &bass.FilePath{"run"},
		},
		Env: manyKeys,
	}

	name, err := workload.SHA1()
	is.NoErr(err)

	// this is a bit silly, but it's deterministic, and we need to make sure it's
	// always the same value
	is.Equal(name, "78f4240582a43dc25aa9f92ca0fb5edb5cf07e06")
}
