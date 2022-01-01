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

const fromStdout = "stdout"
const fromExit = "exit"
const fromFilePrefix = "file:"

var input string
var output string
var responseFrom string
var responseProto string

func init() {
	input = os.Getenv("_BASS_INPUT")
	os.Unsetenv("_BASS_INPUT")

	output = os.Getenv("_BASS_OUTPUT")
	os.Unsetenv("_BASS_OUTPUT")

	responseFrom = os.Getenv("_BASS_RESPONSE_SOURCE")
	os.Unsetenv("_BASS_RESPONSE_SOURCE")

	responseProto = os.Getenv("_BASS_RESPONSE_PROTOCOL")
	os.Unsetenv("_BASS_RESPONSE_PROTOCOL")
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

	response, err := os.Create(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create output error: %s\n", err)
		return 1
	}

	defer response.Close()

	var responseW io.Writer = response

	var protoW io.Writer
	switch responseProto {
	case "json":
		protoW = responseW

	case "unix-table":
		tw := &unixTableWriter{
			enc: json.NewEncoder(responseW),
		}
		defer tw.Flush()

		protoW = tw
	}

	var stdout io.Writer = os.Stdout
	if responseFrom == fromStdout {
		stdout = protoW
	}

	bin := os.Args[1]
	argv := os.Args[2:]
	cmd := exec.Command(bin, argv...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if responseFrom == fromExit {
				if err := json.NewEncoder(protoW).Encode(exit.ExitCode()); err != nil {
					fmt.Fprintf(os.Stderr, "encode exit code error: %s\n", err)
					return 1
				}

				// mask exit status; the point is to determine whether the command
				// succeeds, so that's not a failure
				return 0
			} else {
				// propagate exit status
				return exit.ExitCode()
			}
		} else {
			fmt.Fprintf(os.Stderr, "run error: %s\n", err)
			return 1
		}
	}

	if responseFrom == fromExit {
		if err := json.NewEncoder(protoW).Encode(0); err != nil {
			fmt.Fprintf(os.Stderr, "encode exit code error: %s\n", err)
			return 1
		}
	} else if strings.HasPrefix(responseFrom, fromFilePrefix) {
		responsePath := strings.TrimPrefix(responseFrom, fromFilePrefix)
		origRes, err := os.Open(responsePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open response file error: %s\n", err)
			return 1
		}

		defer origRes.Close()

		_, err = io.Copy(protoW, origRes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "write response file error: %s\n", err)
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
