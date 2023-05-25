package cli

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/morikuni/aec"
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

func (progErr ProgressError) NiceError(w io.Writer, outer error) error {
	fmt.Fprintln(w, aec.RedF.Apply(outer.Error()))
	fmt.Fprintln(w)

	fmt.Fprintln(w, "run summary:")
	fmt.Fprintln(w)
	progErr.prog.Summarize(textio.NewPrefixWriter(w, "  "))

	fmt.Fprintln(w)
	fmt.Fprintln(w, aec.YellowF.Apply("for more information, refer to the full output above"))

	return nil
}

var processErr = regexp.MustCompile(`process ".*" did not complete successfully: `)

func stripUselessPart(msg string) string {
	return processErr.ReplaceAllString(msg, "")
}

func isCanceled(msg string) bool {
	return strings.HasSuffix(msg, context.Canceled.Error())
}
