package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/alecthomas/chroma/formatters"
	"github.com/morikuni/aec"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/hl"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/std"
)

func WriteError(ctx context.Context, err error) {
	out := ioctx.StderrFromContext(ctx)

	trace, found := bass.TraceFrom(ctx)
	if found && !errors.Is(err, bass.ErrInterrupted) {
		if !trace.IsEmpty() {
			WriteTrace(ctx, out, trace)
			trace.Reset()
		}
	}

	if nice, ok := err.(bass.NiceError); ok {
		metaErr := nice.NiceError(out)
		if metaErr != nil {
			fmt.Fprintf(out, aec.RedF.Apply("errored while erroring: %s")+"\n", metaErr)
			fmt.Fprintf(out, aec.RedF.Apply("original error: %T: %s")+"\n", err, err)
		}
	} else if readErr, ok := err.(bass.ReadError); ok {
		Annotate(ctx, out, readErr.Range)
		fmt.Fprintf(out, aec.RedF.Apply("%s")+"\n", err)
	} else {
		fmt.Fprintf(out, aec.RedF.Apply("%s")+"\n", err)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Tip: if this error is too cryptic, please open an issue:")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  https://github.com/vito/bass/issues/new?labels=cryptic&template=cryptic-error-message.md")
	}
}

func WriteTrace(ctx context.Context, out io.Writer, trace *bass.Trace) {
	frames := trace.Frames()

	fmt.Fprintln(out, aec.YellowF.Apply("error!")+" call trace (oldest first):")
	fmt.Fprintln(out)

	elided := 0
	for _, frame := range frames {
		var fsp bass.FSPath
		if err := frame.Range.File.Decode(&fsp); err == nil && fsp.ID == std.FSID && fsp.Path.String() == "./root.bass" {
			elided++
			continue
		}

		numLen := int(math.Log10(float64(frame.Range.End.Ln))) + 1
		if numLen < 2 {
			numLen = 2
		}

		pad := strings.Repeat(" ", numLen)

		if elided > 0 {
			if elided == 1 {
				fmt.Fprintf(out, aec.LightBlackF.Apply("%s ┆ (1 internal call elided)")+"\n", pad)
			} else {
				fmt.Fprintf(out, aec.LightBlackF.Apply("%s ┆ (%d internal calls elided)")+"\n", pad, elided)
			}

			elided = 0
		}

		Annotate(ctx, out, frame.Range)
	}
}

func Annotate(ctx context.Context, out io.Writer, loc bass.Range) {
	numLen := int(math.Log10(float64(loc.End.Ln))) + 1
	if numLen < 2 {
		numLen = 2
	}

	pad := strings.Repeat(" ", numLen)

	fmt.Fprintf(out, aec.YellowF.Apply("%s ┆ %s")+"\n", pad, loc)

	f, err := loc.File.Open(ctx)
	if err != nil {
		fmt.Fprintf(out, aec.RedF.Apply("%s ! could not open frame source: %s")+"\n", pad, err)
		fmt.Fprintln(out)
		return
	}

	defer f.Close()

	scan := bufio.NewScanner(f)

	startLn := loc.Start.Ln
	endLn := loc.End.Ln

	if endLn != startLn {
		startLn -= 1
		if startLn < 1 {
			startLn = 1
		}
	}

	var maxErrLen int
	for ln := 1; ln <= endLn; ln++ {
		if !scan.Scan() {
			break
		}

		line := scan.Text()

		if ln < startLn || ln > endLn {
			continue
		}

		linePrefix := fmt.Sprintf("%[2]*[1]d │ ", ln, numLen)
		if ln >= loc.Start.Ln && ln <= loc.End.Ln {
			if len(line) > maxErrLen {
				maxErrLen = len(line)
			}

			fmt.Fprintf(out, aec.RedF.Apply("%s"), linePrefix)
		} else {
			fmt.Fprintf(out, "%s", linePrefix)
		}

		tokens, err := hl.BassLexer.Tokenise(nil, line)
		if err != nil {
			fmt.Fprintf(out, "tokenize error: %s\n", err)
			continue
		}

		err = formatters.TTY16.Format(out, hl.TTYStyle, tokens)
		if err != nil {
			fmt.Fprintf(out, "format error: %s\n", err)
			continue
		}

		fmt.Fprintln(out)

		if ln == loc.End.Ln {
			startCol := loc.Start.Col

			var endCol int = len(line)
			if ln == loc.End.Ln {
				endCol = loc.End.Col
			}

			carets := endCol - startCol
			if maxErrLen > startCol && loc.Start.Ln != loc.End.Ln {
				carets = maxErrLen - startCol
			}

			fmt.Fprintf(out,
				"   %s  "+aec.RedF.Apply("%s")+"\n",
				strings.Repeat(" ", startCol),
				strings.Repeat("^", carets))
		}
		continue
	}

	fmt.Fprintln(out)
}
