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
	baseHash, err := a.Hash()
	is.NoErr(err)

	diffContent, err := NewInMemoryFSDir(
		FilePath{"top-file"}, String("x2"),
		FilePath{"sub/file"}, String("y2"),
	)
	is.NoErr(err)
	diffContentHash, err := diffContent.Hash()
	is.NoErr(err)

	diffName, err := NewInMemoryFSDir(
		FilePath{"top-file2"}, String("x"),
		FilePath{"sub/file2"}, String("y"),
	)
	is.NoErr(err)
	diffNameHash, err := diffName.Hash()
	is.NoErr(err)

	// distinguish file name from content
	diffName2, err := NewInMemoryFSDir(
		FilePath{"top-file"}, String("2x"),
		FilePath{"sub/file"}, String("2y"),
	)
	is.NoErr(err)
	diffName2Hash, err := diffName2.Hash()
	is.NoErr(err)

	is.True(baseHash != diffContentHash)
	is.True(baseHash != diffNameHash)
	is.True(diffNameHash != diffName2Hash)
}
