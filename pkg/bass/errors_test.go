package bass_test

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"

	"github.com/morikuni/aec"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/is"
)

func TestUnboundErrorNice(t *testing.T) {
	is := is.New(t)

	type output []string

	for _, example := range []struct {
		Symbol   bass.Symbol
		Bindings bass.Bindings
		Message  output
	}{
		{
			"foo",
			bass.Bindings{},
			[]string{
				`unbound symbol: foo`,
			},
		},
		{
			"f123",
			bass.Bindings{
				"f1234567890": bass.Null{},
				"f12345678":   bass.Null{},
				"f123456":     bass.Null{},
				"f1234":       bass.Null{},
				"f12":         bass.Null{},
			},
			output{
				`unbound symbol: f123`,
				``,
				`similar bindings:`,
				``,
				fmt.Sprintf(`* %s`, aec.Bold.Apply("f1234")),
				fmt.Sprintf(`* %s`, aec.Bold.Apply("f12")),
				fmt.Sprintf(`* %s`, aec.Faint.Apply("f123456")),
				``,
				fmt.Sprintf(`did you mean %s, perchance?`, aec.Bold.Apply("f1234")),
			},
		},
	} {
		example := example
		scope := example.Bindings.Scope()

		t.Run(fmt.Sprintf("%s with %s", example.Symbol, scope), func(t *testing.T) {
			is := is.New(t)

			unboundErr := bass.UnboundError{
				Symbol: example.Symbol,
				Scope:  scope,
			}

			out := new(bytes.Buffer)
			is.NoErr(unboundErr.NiceError(out))

			scanner := bufio.NewScanner(out)
			for _, line := range example.Message {
				if !scanner.Scan() {
					t.Error("EOF")
					break
				}

				is.Equal(line, scanner.Text())
			}

			for scanner.Scan() {
				t.Errorf("extra output: %q", scanner.Text())
			}
		})
	}
}
