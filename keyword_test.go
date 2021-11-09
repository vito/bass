package bass_test

import (
	"testing"

	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/is"
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

	Equal(t, bass.Keyword("hello"), bass.Keyword("hello"))
	is.True(!bass.Keyword("hello").Equal(bass.String("hello")))
	Equal(t, bass.Keyword("hello"), wrappedValue{bass.Keyword("hello")})
	is.True(!bass.Keyword("hello").Equal(wrappedValue{bass.String("hello")}))
}
