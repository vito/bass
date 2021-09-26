package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	prompt "github.com/c-bata/go-prompt"
	"github.com/mattn/go-colorable"
	"github.com/spy16/slurp/reader"
	"github.com/vito/bass"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/runtimes"
	"golang.org/x/term"
)

const promptStr = "=> "
const wordsep = "()[]{} "

const complColor = prompt.Green
const textColor = prompt.White

type Session struct {
	ctx context.Context

	scope *bass.Scope
	read  *bass.Reader

	partial      *bytes.Buffer
	partialDepth int
}

func repl(ctx context.Context) error {
	ctx = ioctx.StderrToContext(ctx, colorable.NewColorableStderr())

	scope := runtimes.NewScope(bass.Ground, runtimes.RunState{
		Dir:    bass.HostPath{Path: "."},
		Args:   bass.NewList(),
		Stdin:  bass.Stdin,
		Stdout: bass.Stdout,
	})

	buf := new(bytes.Buffer)
	session := &Session{
		ctx: ctx,

		scope: scope,
		read:  bass.NewReader(buf, "(repl)"),

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

	fd := int(os.Stdin.Fd())
	before, err := term.GetState(fd)
	if err != nil {
		return err
	}

	p.Run()

	// restore terminal state manually; for some reason go-prompt doesn't restore
	// isig which breaks Ctrl+C
	return term.Restore(fd, before)
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

		rdy := form.Eval(session.ctx, session.scope, bass.Identity)

		res, err := bass.Trampoline(session.ctx, rdy)
		if err != nil {
			bass.WriteError(session.ctx, err)
			continue
		}

		var wl bass.Workload
		if err := res.Decode(&wl); err == nil {
			avatar, err := wl.Avatar()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				fmt.Fprint(os.Stdout, avatar)
			}
		}

		fmt.Fprintln(os.Stdout, res)
	}
}

func (session *Session) Complete(doc prompt.Document) []prompt.Suggest {
	word := doc.GetWordBeforeCursorUntilSeparator(wordsep)
	if word == "" {
		return nil
	}

	suggestions := []prompt.Suggest{}
	for _, opt := range session.scope.Complete(word) {
		var desc string
		if opt.Value.Comment != "" {
			desc = strings.Split(opt.Value.Comment, "\n\n")[0]
		} else {
			desc = bass.Details(opt.Value.Value)
		}

		suggestions = append(suggestions, prompt.Suggest{
			Text:        opt.Binding.String(),
			Description: desc,
		})
	}

	return suggestions
}

func (session *Session) Prefix() (string, bool) {
	if session.partialDepth == 0 {
		return "", false
	}

	return strings.Repeat(" ", len(promptStr)) +
		strings.Repeat("  ", session.partialDepth), true
}
