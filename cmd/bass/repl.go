package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	prompt "github.com/c-bata/go-prompt"
	"github.com/spy16/slurp/reader"
	"github.com/vito/bass"
)

const promptStr = "=> "
const wordsep = "()[]{} "

const complColor = prompt.Green
const textColor = prompt.White

type Session struct {
	env  *bass.Env
	read *bass.Reader

	partial      *bytes.Buffer
	partialDepth int
}

func repl(env *bass.Env) error {
	buf := new(bytes.Buffer)
	session := &Session{
		env:  env,
		read: bass.NewReader(buf),

		partial: buf,
	}

	p := prompt.New(
		session.ReadLine,
		session.Complete,
		prompt.OptionPrefix(promptStr),
		prompt.OptionLivePrefix(session.Prefix),
		prompt.OptionCompletionWordSeparator(wordsep),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
		prompt.OptionSuggestionTextColor(complColor),
		prompt.OptionDescriptionBGColor(prompt.DarkGray),
		prompt.OptionDescriptionTextColor(textColor),
		prompt.OptionSelectedSuggestionBGColor(complColor),
		prompt.OptionSelectedSuggestionTextColor(prompt.Black),
		prompt.OptionSelectedDescriptionBGColor(complColor),
		prompt.OptionSelectedDescriptionTextColor(prompt.Black),
		prompt.OptionPreviewSuggestionTextColor(complColor),
		prompt.OptionPrefixTextColor(prompt.Purple),
		prompt.OptionScrollbarBGColor(prompt.DarkGray),
		prompt.OptionScrollbarThumbColor(prompt.White))

	p.Run()

	return nil
}

func (session *Session) ReadLine(in string) {
	buf := session.partial

	fmt.Fprintln(session.partial, in)

	content := session.partial.String()

	for {
		form, err := session.read.Next()
		if err != nil {
			if errors.Is(err, reader.ErrEOF) {
				// XXX: subtle gotcha: content here will be the *entire line* even
				// if preceding expressions have already been read and evaluated,
				// so once the expression is completed the preceding expressions
				// will be evaluated again
				//
				// ideally this would be fixed by capturing the buffer before
				// calling .Next, however slurp wraps the reader in a bufio reader,
				// so it reads more from the reader than it actually processes.
				_, _ = fmt.Fprintf(buf, "%s\n", content)
				opens := strings.Count(buf.String(), "(")
				opens += strings.Count(buf.String(), "[")
				closes := strings.Count(buf.String(), ")")
				closes += strings.Count(buf.String(), "]")
				session.partialDepth = opens - closes
				if session.partialDepth < 0 {
					// TODO: is this possible? feel like that wouldn't be an EOF
					session.partialDepth = 0
				}
				return
			} else {
				if err == io.EOF {
					return
				} else {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
			}
		}

		session.partialDepth = 0

		rdy := form.Eval(session.env, bass.Identity)

		res, err := bass.Trampoline(rdy)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		fmt.Fprintln(os.Stdout, res)
	}
}

func (session *Session) Complete(doc prompt.Document) []prompt.Suggest {
	return completeEnv(session.env, doc)
}

func (session *Session) Prefix() (string, bool) {
	if session.partialDepth == 0 {
		return "", false
	}

	return strings.Repeat(" ", len(promptStr)) +
		strings.Repeat("  ", session.partialDepth), true
}

func completeEnv(env *bass.Env, doc prompt.Document) []prompt.Suggest {
	word := doc.GetWordBeforeCursorUntilSeparator(wordsep)
	if word == "" {
		return nil
	}

	suggestions := []prompt.Suggest{}

	for name, val := range env.Bindings {
		if strings.HasPrefix(string(name), word) {
			var desc string

			doc, found := env.Docs[name]
			if found {
				desc = strings.Split(doc, "\n\n")[0]
			} else {
				desc = fmt.Sprintf("binding (%T)", val)
			}

			suggestions = append(suggestions, prompt.Suggest{
				Text:        string(name),
				Description: desc,
			})
		}
	}

	// sort before appending parent bindings, so order shows local bindings first
	sort.Sort(options(suggestions[:]))

	// TODO: omit suggestions for shadowed bindings
	for _, parent := range env.Parents {
		suggestions = append(suggestions, completeEnv(parent, doc)...)
	}

	return suggestions
}

type options []prompt.Suggest

func (opts options) Len() int      { return len(opts) }
func (opts options) Swap(i, j int) { opts[i], opts[j] = opts[j], opts[i] }
func (opts options) Less(i, j int) bool {
	if len(opts[i].Text) < len(opts[j].Text) {
		return true
	}

	if len(opts[i].Text) > len(opts[j].Text) {
		return false
	}

	return opts[i].Text < opts[j].Text
}
