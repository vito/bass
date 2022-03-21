package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
)

// overridden by ldflags
var version string = "dev"

func printVersion(ctx context.Context) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Fprintln(os.Stderr, "impossible: build info unavailable")
		os.Exit(1)
	}

	var rev, date string
	var dirty bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			rev = setting.Value
		case "vcs.time":
			date = setting.Value
		case "vcs.modified":
			if setting.Value == "true" {
				dirty = true
			}
		}
	}

	if dirty && rev != "" {
		rev += "*"
	}

	fmt.Printf("bass\t%s\n", version)

	if rev != "" {
		fmt.Printf("commit\t%s\n", rev)
	}

	if date != "" {
		fmt.Printf("date\t%s\n", date)
	}
}
