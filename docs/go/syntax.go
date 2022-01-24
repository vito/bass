package plugin

import (
	"bytes"
	"regexp"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/vito/booklit"
)

func (plugin *Plugin) Syntax(language string, code booklit.Content) (booklit.Content, error) {
	return plugin.SyntaxTransform(language, code, styles.Fallback)
}

func (plugin *Plugin) Bass(code booklit.Content) (booklit.Content, error) {
	return plugin.SyntaxTransform("bass", code, styles.Fallback)
}

type Transformer struct {
	Pattern   *regexp.Regexp
	Transform func(string) booklit.Content
}

func (t Transformer) TransformAll(str string) booklit.Sequence {
	matches := t.Pattern.FindAllStringIndex(str, -1)

	out := booklit.Sequence{}
	last := 0
	for _, match := range matches {
		if match[0] > last {
			out = append(out, booklit.String(str[last:match[0]]))
		}

		out = append(out, t.Transform(str[match[0]:match[1]]))

		last = match[1]
	}

	if len(str) > last {
		out = append(out, booklit.String(str[last:]))
	}

	return out
}

func (plugin Plugin) SyntaxTransform(language string, code booklit.Content, chromaStyle *chroma.Style, transformers ...Transformer) (booklit.Content, error) {
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	iterator, err := lexer.Tokenise(nil, code.String())
	if err != nil {
		return nil, err
	}

	formatter := html.New(
		html.PreventSurroundingPre(code.IsFlow()),
		html.WithClasses(true),
	)

	buf := new(bytes.Buffer)
	err = formatter.Format(buf, chromaStyle, iterator)
	if err != nil {
		return nil, err
	}

	var style booklit.Style
	if code.IsFlow() {
		style = "code-flow"
	} else {
		style = "code-block"
	}

	highlighted := booklit.Sequence{booklit.String(buf.String())}

	for _, t := range transformers {
		var newHighlighted booklit.Sequence
		for _, con := range highlighted {
			switch val := con.(type) {
			case booklit.String:
				newHighlighted = append(newHighlighted, t.TransformAll(val.String())...)
			default:
				newHighlighted = append(newHighlighted, con)
			}
		}

		highlighted = newHighlighted
	}

	for i, con := range highlighted {
		if _, ok := con.(booklit.String); ok {
			highlighted[i] = booklit.Styled{
				Style:   "raw-html",
				Content: con,
			}
		}
	}

	return booklit.Styled{
		Style:   style,
		Block:   !code.IsFlow(),
		Content: highlighted,
		Partials: booklit.Partials{
			"Language": booklit.String(language),
		},
	}, nil
}
