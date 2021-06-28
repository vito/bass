package main

import (
	"fmt"
	"os"

	"github.com/vito/bass"
)

func main() {
	switch len(os.Args) {
	case 1:
		err := repl(bass.New())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
