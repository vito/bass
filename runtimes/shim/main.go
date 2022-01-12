package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

var input string
var output string

func init() {
	input = os.Getenv("_BASS_INPUT")
	os.Unsetenv("_BASS_INPUT")

	output = os.Getenv("_BASS_OUTPUT")
	os.Unsetenv("_BASS_OUTPUT")
}

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "usage: %s command...", os.Args[0])
		return 1
	}

	stdin := os.Stdin
	if input != "" {
		var err error
		stdin, err = os.Open(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read input error: %s\n", err)
			return 1
		}

		defer stdin.Close()
	}

	var stdout io.Writer = os.Stdout
	if output != "" {
		response, err := os.Create(output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create output error: %s\n", err)
			return 1
		}

		defer response.Close()

		stdout = response
	}

	bin := os.Args[1]
	argv := os.Args[2:]
	cmd := exec.Command(bin, argv...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			// propagate exit status
			return exit.ExitCode()
		} else {
			fmt.Fprintf(os.Stderr, "run error: %s\n", err)
			return 1
		}
	}

	return 0
}

type unixTableWriter struct {
	enc *json.Encoder
	buf []byte
}

func (w *unixTableWriter) Write(p []byte) (int, error) {
	written := len(p)

	for len(p) > 0 {
		if w.buf != nil {
			cp := []byte{}
			cp = append(cp, w.buf...)
			cp = append(cp, p...)
			p = cp
			w.buf = nil
		}

		ln := bytes.IndexRune(p, '\n')
		if ln == -1 {
			cp := []byte{}
			cp = append(cp, p...)
			w.buf = cp
			break
		}

		row := string(p[:ln])

		err := w.enc.Encode(strings.Fields(row))
		if err != nil {
			return 0, err
		}

		p = p[ln+1:]
	}

	return written, nil
}

func (w unixTableWriter) Flush() error {
	if len(w.buf) > 0 {
		return w.enc.Encode(strings.Fields(string(w.buf)))
	}

	return nil
}
