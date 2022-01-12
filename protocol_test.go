package bass_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/vito/bass"
	"github.com/vito/is"
)

func TestUnixTableProtocol(t *testing.T) {
	is := is.New(t)

	proto := bass.UnixTableProtocol{}

	out := new(bytes.Buffer)
	log := new(bytes.Buffer)
	w := proto.ResponseWriter(out, log)

	fmt.Fprint(w, "a 1\n")
	is.Equal(`["a","1"]
`, out.String())
	out.Reset()

	fmt.Fprint(w, "b\t2\n")
	is.Equal(`["b","2"]
`, out.String())
	out.Reset()

	fmt.Fprint(w, "c   3  \n")
	is.Equal(`["c","3"]
`, out.String())
	out.Reset()

	fmt.Fprint(w, "\n")
	is.Equal(`[]
`, out.String())
	out.Reset()

	fmt.Fprint(w, "d ")
	is.Equal(``, out.String()) // no linebreak
	fmt.Fprint(w, " 4 ")
	is.Equal(``, out.String()) // still no linebreak
	fmt.Fprint(w, " done\n")
	is.Equal(`["d","4","done"]
`, out.String())
	out.Reset()

	fmt.Fprint(w, "e a sports\ntsin the game\n")
	is.Equal(`["e","a","sports"]
["tsin","the","game"]
`, out.String())
	out.Reset()

	fmt.Fprint(w, "unfinished busine")
	is.Equal(``, out.String()) // no linebreak
	w.Flush()
	is.Equal(`["unfinished","busine"]
`, out.String())
}
