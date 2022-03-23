package plugin

import (
	"strings"

	"github.com/vito/booklit"
)

func (plugin *Plugin) T(content booklit.Content) booklit.Content {
	return &booklit.Reference{
		TagName: "term-" + plugin.plural.Singular(strings.ToLower(strings.ReplaceAll(content.String(), "'", ""))),
		Content: content,
	}
}

func (plugin *Plugin) B(content booklit.Content) booklit.Content {
	return &booklit.Reference{
		TagName: "binding-" + content.String(),
	}
}

func (plug *Plugin) Term(term, definition booklit.Content, literate ...booklit.Content) error {
	body := append([]booklit.Content{definition}, literate...)
	prose, err := plug.BassLiterate(body...)
	if err != nil {
		return err
	}

	section := &booklit.Section{
		Style: "bass-term",

		Parent: plug.Section,

		Title: booklit.Styled{
			Style:   "term",
			Content: term,
		},
		Body: prose,

		Processor:     plug.Section.Processor,
		Location:      plug.Section.InvokeLocation,
		TitleLocation: plug.Section.InvokeLocation,
	}

	tag := booklit.Tag{
		Name:     "term-" + term.String(),
		Title:    section.Title,
		Section:  section,
		Content:  definition,
		Location: plug.Section.InvokeLocation,
	}

	section.PrimaryTag = tag
	section.Tags = []booklit.Tag{tag}

	plug.Section.Children = append(plug.Section.Children, section)

	return nil
}

func (*Plugin) SideBySide(content ...booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "side-by-side",
		Content: booklit.Sequence(content),
	}
}

func (*Plugin) Construction(content booklit.Content) booklit.Content {
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
