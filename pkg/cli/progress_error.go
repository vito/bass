package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/morikuni/aec"
	"github.com/opencontainers/go-digest"
	perrors "github.com/pkg/errors"
	"github.com/segmentio/textio"
)

type ProgressError struct {
	msg  string
	err  error
	prog *Progress
}

func (err ProgressError) Error() string {
	rootErr := perrors.Cause(err.err)
	return fmt.Sprintf("%s: %s", err.msg, stripUselessPart(rootErr.Error()))
}

func (progErr ProgressError) NiceError(w io.Writer) error {
	fmt.Fprintf(w, aec.RedF.Apply("%s")+"\n", progErr.Error())
	fmt.Fprintln(w)

	progErr.prog.Summarize(textio.NewPrefixWriter(w, "  "))

	fmt.Fprintln(w)
	fmt.Fprintln(w, aec.YellowF.Apply("for more information, refer to the full output above"))

	return nil
}

func duration(dt time.Duration) string {
	prec := 1
	sec := dt.Seconds()
	if sec < 10 {
		prec = 2
	} else if sec < 100 {
		prec = 1
	}

	return fmt.Sprintf(aec.Faint.Apply("[%.[2]*[1]fs]"), sec, prec)
}

type vtxPrinter struct {
	vs      map[digest.Digest]*Vertex
	printed map[digest.Digest]struct{}
}

func (printer vtxPrinter) printAll(w io.Writer) {
	byStartTime := make([]*Vertex, 0, len(printer.vs))
	for _, vtx := range printer.vs {
		byStartTime = append(byStartTime, vtx)
	}
	sort.Slice(byStartTime, func(i, j int) bool {
		if byStartTime[i].Started != nil && byStartTime[j].Started == nil {
			return true
		}

		if byStartTime[i].Started == nil && byStartTime[j].Started != nil {
			return false
		}

		if byStartTime[i].Started == nil && byStartTime[j].Started == nil {
			return byStartTime[i].Name < byStartTime[j].Name
		}

		return byStartTime[i].Started.Before(*byStartTime[j].Started)
	})

	for _, vtx := range byStartTime {
		printer.print(w, vtx)
	}
}

const maxLines = 24

func (printer vtxPrinter) print(w io.Writer, vtx *Vertex) error {
	if _, printed := printer.printed[vtx.Digest]; printed {
		return nil
	}

	printer.printed[vtx.Digest] = struct{}{}

	if strings.Contains(vtx.Name, "[hide]") {
		return nil
	}

	for _, input := range vtx.Inputs {
		iv, found := printer.vs[input]
		if !found {
			continue
		}

		printer.print(w, iv)
	}

	if vtx.Cached {
		fmt.Fprintf(w, aec.BlueF.Apply("=> %s"), vtx.Name)
	} else if vtx.Error != "" {
		if strings.HasSuffix(vtx.Error, context.Canceled.Error()) {
			fmt.Fprintf(w, aec.YellowF.Apply("=> %s [canceled]")+" %s", vtx.Name, duration(vtx.Completed.Sub(*vtx.Started)))
		} else {
			fmt.Fprintf(w, aec.RedF.Apply("=> %s")+" %s", vtx.Name, duration(vtx.Completed.Sub(*vtx.Started)))
		}
	} else if vtx.Started == nil {
		fmt.Fprintf(w, aec.Faint.Apply("=> %s"), vtx.Name)
	} else if vtx.Completed != nil {
		fmt.Fprintf(w, aec.GreenF.Apply("=> %s")+" %s", vtx.Name, duration(vtx.Completed.Sub(*vtx.Started)))
	} else {
		fmt.Fprintf(w, aec.YellowF.Apply("=> %s"), vtx.Name)
	}

	fmt.Fprintln(w)

	if vtx.Error != "" {
		if vtx.Log.Len() > 0 {
			trimTrail := bytes.TrimRight(vtx.Log.Bytes(), "\n")
			lines := bytes.Split(trimTrail, []byte("\n"))
			if len(lines) > maxLines {
				extra := len(lines) - maxLines
				lines = lines[len(lines)-maxLines-1:]
				fmt.Fprintf(w, aec.Faint.Apply("... %d lines omitted ...")+"\n", extra)
			}

			for _, line := range lines {
				fmt.Fprintln(w, string(line))
			}
		}

		if !isCanceled(vtx.Error) {
			fmt.Fprintf(w, aec.RedF.Apply("ERROR: %s")+"\n", stripUselessPart(vtx.Error))
		}
	}

	return nil
}

var processErr = regexp.MustCompile(`process ".*" did not complete successfully: `)

func stripUselessPart(msg string) string {
	return processErr.ReplaceAllString(msg, "")
}

func isCanceled(msg string) bool {
	return strings.HasSuffix(msg, context.Canceled.Error())
}
