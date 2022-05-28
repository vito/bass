package bass

import (
	"testing"

	"github.com/vito/is"
)

func TestFSPathEqual(t *testing.T) {
	is := is.New(t)
	is.True(NewInMemoryFile("name-also-id", "a").Equal(NewInMemoryFile("name-also-id", "b")))
	is.True(!NewInMemoryFile("name-also-id", "a").Equal(NewInMemoryFile("name-also-id-other", "b")))
}
