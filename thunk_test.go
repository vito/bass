package bass_test

import (
	"fmt"
	"testing"

	"github.com/vito/bass"
	"github.com/vito/is"
)

func TestThunkName(t *testing.T) {
	is := is.New(t)

	// use an object with a ton of keys to test stable order when hashing
	manyKeys := bass.NewEmptyScope()
	for i := 0; i < 100; i++ {
		manyKeys.Set(bass.Symbol(fmt.Sprintf("key-%d", i)), bass.Int(i))
	}

	thunk := bass.Thunk{
		Cmd: bass.ThunkCmd{
			File: &bass.FilePath{"run"},
		},
		Env: manyKeys,
	}

	name, err := thunk.SHA1()
	is.NoErr(err)

	// this is a bit silly, but it's deterministic, and we need to make sure it's
	// always the same value
	is.Equal(name, "59e4d21b8dcfaa6a6d6d563157c188d1de516852")
}
