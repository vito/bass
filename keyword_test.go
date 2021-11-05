package bass_test

import (
	"testing"

	"github.com/matryer/is"
	"github.com/vito/bass"
)

func TestKeywordDecode(t *testing.T) {
	is := is.New(t)

	var sym bass.Keyword
	err := bass.Keyword("foo").Decode(&sym)
	is.NoErr(err)
	is.Equal(sym, bass.Keyword("foo"))
}

func TestKeywordEqual(t *testing.T) {
	is := is.New(t)

	is.True(bass.Keyword("hello").Equal(bass.Keyword("hello")))
	is.True(!bass.Keyword("hello").Equal(bass.String("hello")))
	is.True(bass.Keyword("hello").Equal(wrappedValue{bass.Keyword("hello")}))
	is.True(!bass.Keyword("hello").Equal(wrappedValue{bass.String("hello")}))
}
