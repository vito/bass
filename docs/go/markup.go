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

func (plug *Plugin) Term(term, definition booklit.Content, literate ...booklit.Content) (booklit.Content, error) {
	name := "term-" + term.String()

	target := booklit.Target{
		TagName:  name,
		Location: plug.Section.InvokeLocation,
		Title: booklit.Styled{
			Style:   booklit.StyleBold,
			Content: term,
		},
		Content: definition,
	}

	body := append([]booklit.Content{definition}, literate...)
	prose, err := plug.BassLiterate(body...)
	if err != nil {
		return nil, err
	}

	return booklit.Styled{
		Style:   "bass-term",
		Content: prose,
		Partials: booklit.Partials{
			"Reference": &booklit.Reference{
				TagName: name,
			},
			"Target": target,
		},
	}, nil
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
