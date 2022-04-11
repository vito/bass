package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"
	"time"

	"github.com/adrg/xdg"
	"github.com/c-bata/go-prompt"
	"github.com/spy16/slurp/reader"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/progrock"
	"golang.org/x/term"
)

const promptStr = "=> "
const wordsep = "()[]{} "

const complColor = prompt.Green
const textColor = prompt.White

func Repl(ctx context.Context) error {
	env := bass.ImportSystemEnv()

	scope := runtimes.NewScope(bass.Ground, runtimes.RunState{
		Dir:    bass.NewHostDir("."),
		Stdin:  bass.Stdin,
		Stdout: bass.Stdout,
		Env:    env,
	})

	buf := new(bytes.Buffer)
	session := &ReplSession{
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
		WriteError(ctx, err)
		return err
	}

	p.Run()

	// restore terminal state manually; for some reason go-prompt doesn't restore
	// isig which breaks Ctrl+C
	return term.Restore(fd, before)
}

type ReplSession struct {
	ctx context.Context

	scope *bass.Scope
	read  *bass.Reader

	partial *bytes.Buffer
}

func (session *ReplSession) ReadLine(in string) {
	if err := appendHistory(in); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to append to history: %s\n", err)
	}

	buf := session.partial

	fmt.Fprintln(session.partial, in)

	content := session.partial.String()

	recordRepl(content)

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

				// un-read content
				buf.Reset()
				_, _ = fmt.Fprint(buf, content)

				return
			} else {
				if err == io.EOF {
					return
				} else {
					WriteError(session.ctx, err)
					continue
				}
			}
		}

		statuses, w := progrock.Pipe()
		recorder := progrock.NewRecorder(w)
		evalCtx, cancel := context.WithCancel(progrock.RecorderToContext(session.ctx, recorder))

		ui := ProgressUI
		ui.ConsoleRunning = ""
		ui.ConsoleDone = ""
		recorder.Display(cancel, ui, os.Stderr, statuses, false)

		res, err := bass.Trampoline(evalCtx, form.Eval(evalCtx, session.scope, bass.Identity))
		if err != nil {
			WriteError(session.ctx, err)
			recorder.Stop()
			continue
		}

		recorder.Stop()

		var wl bass.Thunk
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

func (session *ReplSession) Complete(doc prompt.Document) []prompt.Suggest {
	word := doc.GetWordBeforeCursorUntilSeparator(wordsep)
	if word == "" {
		return nil
	}

	suggestions := []prompt.Suggest{}
	for _, opt := range session.scope.Complete(word) {
		var desc string

		var doc string
		if err := opt.Value.Meta.GetDecode(bass.DocMetaBinding, &doc); err == nil {
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

func (session *ReplSession) Prefix() (string, bool) {
	if session.partial.Len() == 0 {
		return "", false
	}

	return strings.Repeat(".", len(promptStr)), true
}

var ReplFS fs.FS = fstest.MapFS{
	"(repl)": replFile,
}

var replFile = &fstest.MapFile{
	Data: []byte{},
	Mode: 0644,
}

func recordRepl(line string) {
	replFile.Data = append(replFile.Data, []byte(line)...)
	replFile.ModTime = time.Now()
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
