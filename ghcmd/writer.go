package ghcmd

import (
	"bytes"
	"io"
)

// Writer handles GitHub workflow commands written to it with a configured
// handler.
//
// Non-command output will be passed through to the inner writer.
//
// The Writer is careful to handle commands that span write chunk boundaries,
// while allowing non-command lines of text to pass through immediately without
// linewise buffering.
type Writer struct {
	// The underlying writer to which non-command output will be written.
	Writer io.Writer

	// The handler to invoke with every command detected.
	Handler Handler

	// in the middle of a non-command line; pass through until next linebreak
	midline bool

	// partial command buffered across byte chunks; parse until linebreak
	partial []byte
}

// Handler is called for every command detected by the writer.
type Handler interface {
	HandleCommand(*Command) error
}

// HandlerFunc is a simple callback for handling commands.
type HandlerFunc func(*Command) error

// HandlerCommand calls the function.
func (f HandlerFunc) HandleCommand(cmd *Command) error {
	return f(cmd)
}

// Dispatch is the special character sequence denoting a command line.
const Dispatch = "::"

// Write processes the given byte slice, buffering as necessary to handle
// commands which span multiple Write calls.
//
// Any commands will be passed to the handler, instead of being written to the
// underlying writer. The handler is responsible for dealing with any unknown
// commands - perhaps by printing it to the underlying writer.
func (w *Writer) Write(p []byte) (int, error) {
	// just pretend we wrote everything like normal even though there's buffering
	// and such
	wroteAll := len(p)

	for len(p) > 0 {
		if w.partial != nil {
			cp := []byte{}
			cp = append(cp, w.partial...)
			cp = append(cp, p...)
			p = cp
			w.partial = nil
		}

		ln := bytes.IndexRune(p, '\n')
		if ln == 0 && len(p) == 1 {
			// special case: blank line
			_, err := w.Writer.Write(p)
			if err != nil {
				return 0, err
			}

			return wroteAll, nil
		}

		if w.midline {
			if ln == -1 {
				// in the middle of a normal line and no end in sight; pass it through
				_, err := w.Writer.Write(p)
				if err != nil {
					return 0, err
				}

				return wroteAll, nil
			}

			// finishing a non-command line
			_, err := w.Writer.Write(p[:ln+1])
			if err != nil {
				return 0, err
			}

			// process the next line
			p = p[ln+1:]
			continue
		}

		if bytes.HasPrefix(p, []byte(Dispatch)) {
			if ln == -1 {
				// partial command; buffer it
				w.partial = p
				return wroteAll, nil
			}

			cmd, err := ParseCommand(string(p[:ln]))
			if err != nil {
				return 0, err
			}

			err = w.Handler.HandleCommand(cmd)
			if err != nil {
				return 0, err
			}

			w.partial = nil
			p = p[ln+1:]
		} else {
			if ln == -1 {
				// regular output, unterminated; pass it through
				w.midline = true
				_, err := w.Writer.Write(p)
				if err != nil {
					return 0, err
				}

				return wroteAll, nil
			}

			_, err := w.Writer.Write(p[:ln+1])
			if err != nil {
				return 0, err
			}

			w.midline = false
			p = p[ln+1:]
		}
	}

	return wroteAll, nil
}
