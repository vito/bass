package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"go.uber.org/zap/zapcore"
)

var debug bool
var logLevel = zapcore.ErrorLevel

func init() {
	if os.Getenv("_BASS_DEBUG") != "" {
		debug = true
		logLevel = zapcore.DebugLevel
		os.Unsetenv("_BASS_DEBUG")
	}
}

var cmds = map[string]func([]string) error{
	"run":        run,
	"unpack":     unpack,
	"get-config": getConfig,
	"check":      check,
}

var cmdArg string

func init() {
	var cmdOpts []string
	for k := range cmds {
		cmdOpts = append(cmdOpts, k)
	}

	sort.Strings(cmdOpts)

	cmdArg = strings.Join(cmdOpts, "|")
}

func main() {
	if len(os.Args) == 1 {
		fmt.Fprintf(os.Stderr, "usage: %s <%s>\n", os.Args[0], cmdArg)
		os.Exit(1)
	}

	cmd, args := os.Args[1], os.Args[2:]

	f, found := cmds[cmd]
	if !found {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		fmt.Fprintf(os.Stderr, "usage: %s <%s>\n", os.Args[0], cmdArg)
		os.Exit(1)
		return
	}

	err := f(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
