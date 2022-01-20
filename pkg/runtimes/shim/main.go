package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var stdoutPath string

type Command struct {
	Args  []string `json:"args"`
	Stdin []byte   `json:"stdin"`
	Env   []string `json:"env"`
	Dir   *string  `json:"dir"`
}

func init() {
	stdoutPath = os.Getenv("_BASS_OUTPUT")
	os.Unsetenv("_BASS_OUTPUT")
}

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	runtime.GOMAXPROCS(1)

	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s cmd.json", args[0])
		return 1
	}

	cmdPath := args[1]

	cmdPayload, err := os.ReadFile(cmdPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read cmd: %s\n", err)
		return 1
	}

	var cmd Command
	err = json.Unmarshal(cmdPayload, &cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unmarshal cmd: %s\n", err)
		return 1
	}

	err = os.Remove(cmdPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "burn after reading: %s\n", err)
		return 1
	}

	var stdout io.Writer = os.Stdout
	if stdoutPath != "" {
		response, err := os.Create(stdoutPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create output error: %s\n", err)
			return 1
		}

		defer response.Close()

		stdout = response
	}

	for _, e := range cmd.Env {
		segs := strings.SplitN(e, "=", 2)
		if len(segs) != 2 {
			fmt.Fprintf(os.Stderr, "warning: malformed env")
			continue
		}

		os.Setenv(segs[0], segs[1])
	}

	bin := cmd.Args[0]
	argv := cmd.Args[1:]
	execCmd := exec.Command(bin, argv...)
	if cmd.Dir != nil {
		execCmd.Dir = *cmd.Dir
	}
	execCmd.Stdin = bytes.NewBuffer(cmd.Stdin)
	execCmd.Stdout = stdout
	execCmd.Stderr = os.Stderr
	err = execCmd.Run()
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
