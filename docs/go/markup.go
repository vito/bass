package plugin

import (
	"strings"

	"github.com/vito/booklit"
)

func (plug *Plugin) Keyword(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "keyword",
		Content: content,
	}
}

func (plug *Plugin) SideBySide(content ...booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "side-by-side",
		Content: booklit.Sequence(content),
	}
}

func (plug *Plugin) Construction(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "construction",
		Content: content,
	}
}

func (plug *Plugin) Commands(commandBlock booklit.Content) (booklit.Content, error) {
	var cmds booklit.Sequence

	lines := strings.Split(commandBlock.String(), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		cmd, err := plug.Syntax("sh", booklit.String(line))
		if err != nil {
			return nil, err
		}

		cmds = append(cmds, cmd)
	}

	return booklit.Styled{
		Style:   "sh-commands",
		Block:   true,
		Content: cmds,
	}, nil
}
