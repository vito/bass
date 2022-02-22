package main

import (
	"context"
	"fmt"
)

// overridden with ldflags
var Version = "dev"
var Commit = ""
var Date = ""

func version(ctx context.Context) {
	version := Version

	if Date != "" && Commit != "" {
		version += " (" + Date + " commit " + Commit + ")"
	}

	fmt.Printf("bass %s\n", version)
}
