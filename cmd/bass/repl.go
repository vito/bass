package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	prompt "github.com/c-bata/go-prompt"
	"github.com/spy16/slurp/reader"
	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/progrock"
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
		prompt.OptionHistory(loadHistory()),
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
		bass.WriteError(ctx, err)
		return err
	}

	p.Run()

	// restore terminal state manually; for some reason go-prompt doesn't restore
	// isig which breaks Ctrl+C
	return term.Restore(fd, before)
}

func (session *Session) ReadLine(in string) {
	if err := appendHistory(in); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to append to history: %s\n", err)
	}

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

		statuses, w := progrock.Pipe()
		recorder := progrock.NewRecorder(w)
		evalCtx := progrock.RecorderToContext(session.ctx, recorder)

		ui := UI
		ui.ConsoleRunning = ""
		ui.ConsoleDone = ""
		recorder.Display(context.Background(), ui, nil, os.Stderr, statuses)

		res, err := bass.Trampoline(evalCtx, form.Eval(evalCtx, session.scope, bass.Identity))
		if err != nil {
			bass.WriteError(session.ctx, err)
			recorder.Stop()
			continue
		}

		recorder.Stop()

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

		var doc string
		if err := opt.Value.Meta.GetDecode("doc", &doc); err == nil {
			desc = strings.Split(doc, "\n\n")[0]
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

func loadHistory() []string {
	logPath, err := xdg.DataFile("bass/history")
	if err != nil {
		return []string{}
	}

	file, err := os.Open(logPath)
	if err != nil {
		return []string{}
	}

	history := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		history = append(history, scanner.Text())
	}

	return history
}

func appendHistory(line string) error {
	logPath, err := xdg.DataFile("bass/history")
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(logPath), 0700)
	if err != nil {
		return err
	}

	history, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(history, line)
	if err != nil {
		return err
	}

	return history.Close()
}
