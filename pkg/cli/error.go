package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/formatters"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/hl"
	"github.com/vito/bass/pkg/ioctx"
)

func WriteError(ctx context.Context, err error) {
	out := ioctx.StderrFromContext(ctx)

	trace, found := bass.TraceFrom(ctx)
	if found && !errors.Is(err, bass.ErrInterrupted) {
		if !trace.IsEmpty() {
			WriteTrace(out, trace)
			trace.Reset()
		}
	}

	if nice, ok := err.(bass.NiceError); ok {
		metaErr := nice.NiceError(out)
		if metaErr != nil {
			fmt.Fprintf(out, "\x1b[31merrored while erroring: %s\x1b[0m\n", metaErr)
			fmt.Fprintf(out, "\x1b[31moriginal error: %T: %s\x1b[0m\n", err, err)
		}
	} else {
		fmt.Fprintf(out, "\x1b[31m%s\x1b[0m\n", err)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Tip: if this error is too cryptic, please open an issue:")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  https://github.com/vito/bass/issues/new?labels=cryptic&template=cryptic-error-message.md")
	}
}

func WriteTrace(out io.Writer, trace *bass.Trace) {
	frames := trace.Frames()

	fmt.Fprintf(out, "\x1b[33merror!\x1b[0m call trace (oldest first):\n\n")

	elided := 0
	for _, frame := range frames {
		if frame.Range.Start.File == "root.bass" {
			elided++
			continue
		}

		numLen := int(math.Log10(float64(frame.Range.End.Ln))) + 1
		if numLen < 2 {
			numLen = 2
		}

		pad := strings.Repeat(" ", numLen)

		// num := len(frames) - i
		if elided > 0 {
			if elided == 1 {
				fmt.Fprintf(out, "\x1b[90m%s ┆ (1 internal call elided)\x1b[0m\n", pad)
			} else {
				fmt.Fprintf(out, "\x1b[90m%s ┆ (%d internal calls elided)\x1b[0m\n", pad, elided)
			}

			elided = 0
		}

		fmt.Fprintf(out, "\x1b[33m%s ┆ %s\x1b[0m\n", pad, frame.Range)

		f, err := frame.Range.Open()
		if err != nil {
			fmt.Fprintf(out, "warning: could not open source file: %s\n", err)
			continue
		}

		defer f.Close()

		scan := bufio.NewScanner(f)

		startLn := frame.Range.Start.Ln
		endLn := frame.Range.End.Ln

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
			if ln >= frame.Range.Start.Ln && ln <= frame.Range.End.Ln {
				if len(line) > maxErrLen {
					maxErrLen = len(line)
				}

				fmt.Fprintf(out, "\x1b[31m%s\x1b[0m", linePrefix)
			} else {
				fmt.Fprintf(out, "\x1b[0m%s\x1b[0m", linePrefix)
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

			if ln == frame.Range.End.Ln {
				startCol := frame.Range.Start.Col

				var endCol int = len(line)
				if ln == frame.Range.End.Ln {
					endCol = frame.Range.End.Col
				}

				carets := endCol - startCol
				if maxErrLen > startCol && frame.Range.Start.Ln != frame.Range.End.Ln {
					carets = maxErrLen - startCol
				}

				fmt.Fprintf(out,
					"   %s  \x1b[31m%s\x1b[0m\n",
					strings.Repeat(" ", startCol),
					strings.Repeat("^", carets))
			}

			continue
		}

		fmt.Fprintln(out)
	}
}

func firstNonSpace(str string) int {
	for i, chr := range str {
		if !unicode.IsSpace(chr) {
			return i
		}
	}

	return -1
}
