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

func TestFSPathHash(t *testing.T) {
	is := is.New(t)
	a, err := NewInMemoryFSDir(
		FilePath{"top-file"}, String("x"),
		FilePath{"sub/file"}, String("y"),
	)
	is.NoErr(err)
	b, err := NewInMemoryFSDir(
		FilePath{"top-file"}, String("x2"),
		FilePath{"sub/file"}, String("y2"),
	)
	ah, err := a.Hash()
	is.NoErr(err)
	bh, err := b.Hash()
	is.NoErr(err)
	is.True(ah != bh)
}
