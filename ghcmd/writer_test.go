package ghcmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vito/bass/ghcmd"
	"github.com/vito/is"
)

type ReaderExample struct {
	Source   []string
	Commands []*ghcmd.Command
	Written  string
}

func TestReader(t *testing.T) {
	for _, example := range []ReaderExample{
		{
			Source:   []string{"hello there\n", "general kenobi"},
			Written:  "hello there\ngeneral kenobi",
			Commands: []*ghcmd.Command{},
		},
		{
			Source:   []string{"hello", " there\n", "\n", "\n\ngeneral ", "kenobi"},
			Written:  "hello there\n\n\n\ngeneral kenobi",
			Commands: []*ghcmd.Command{},
		},
		{
			Source:   []string{"Running tests: ", ".", "..!", "."},
			Written:  "Running tests: ...!.",
			Commands: []*ghcmd.Command{},
		},
		{
			Source:   []string{"::i::am unfini"},
			Written:  "",
			Commands: []*ghcmd.Command{},
		},
		{
			Source: []string{"::i::am finished\n"},
			Commands: []*ghcmd.Command{
				{Name: "i", Params: ghcmd.Params{}, Value: "am finished"},
			},
		},
		{
			Source: []string{"::i::have value\n"},
			Commands: []*ghcmd.Command{
				{Name: "i", Params: ghcmd.Params{}, Value: "have value"},
			},
		},
		{
			Source: []string{"::i have=key::values\n"},
			Commands: []*ghcmd.Command{
				{Name: "i", Params: ghcmd.Params{"have": "key"}, Value: "values"},
			},
		},
		{
			Source: []string{"::i ", "have=", "key", "::values\n"},
			Commands: []*ghcmd.Command{
				{Name: "i", Params: ghcmd.Params{"have": "key"}, Value: "values"},
			},
		},
		{
			Source:   []string{"i am ::malcolm in=middle::and i rule\n"},
			Written:  "i am ::malcolm in=middle::and i rule\n",
			Commands: []*ghcmd.Command{},
		},
	} {
		example.Run(t)
	}
}

func (example ReaderExample) Run(t *testing.T) {
	t.Run(strings.Join(example.Source, ""), func(t *testing.T) {
		is := is.New(t)

		buf := new(bytes.Buffer)
		cmds := []*ghcmd.Command{}

		w := &ghcmd.Writer{
			Writer: buf,
			Handler: ghcmd.HandlerFunc(func(cmd *ghcmd.Command) error {
				cmds = append(cmds, cmd)
				return nil
			}),
		}

		for _, s := range example.Source {
			n, err := w.Write([]byte(s))
			is.NoErr(err)
			is.Equal(n, len(s))
		}

		is.Equal(buf.String(), example.Written)
		is.Equal(cmds, example.Commands)
	})
}
