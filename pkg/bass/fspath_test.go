package bass

import (
	"testing"

	"github.com/vito/is"
)

func TestFSPathEqual(t *testing.T) {
	is := is.New(t)
	a := NewInMemoryFile("name", "a")
	is.True(a.Equal(a))
	cp := *a
	is.True(!a.Equal(&cp))
}
