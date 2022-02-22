package main

import (
	"context"
	"fmt"
	"os"
)

func help(ctx context.Context) {
	fmt.Printf("usage: %s [flags] [scriptfile args...]\n", os.Args[0])
	fmt.Println()
	fmt.Println("runs a bass script, or starts a repl if no args are given")
	fmt.Println()
	fmt.Println("flags:")
	flags.PrintDefaults()
}
