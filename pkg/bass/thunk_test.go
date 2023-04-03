package bass_test

import (
	"fmt"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/is"
)

func TestThunkHash(t *testing.T) {
	is := is.New(t)

	// use an object with a ton of keys to test stable order when hashing
	manyKeys := bass.NewEmptyScope()
	for i := 0; i < 100; i++ {
		manyKeys.Set(bass.Symbol(fmt.Sprintf("ENV_%d", i)), bass.Int(i))
	}

	thunk := bass.Thunk{
		Env: manyKeys,
		Args: []bass.Value{
			bass.FilePath{"run"},
			// ensure HTML characters are not escaped
			bass.String("foo >> bar"),
		},
	}

	hash, err := thunk.Hash()
	is.NoErr(err)

	// this is a bit silly, but it's deterministic, and we need to make sure it's
	// always the same value
	is.Equal(hash, "LCV6HSUTK70GE")
}
