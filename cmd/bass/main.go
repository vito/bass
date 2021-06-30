package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/signal"

	"github.com/mattn/go-colorable"
	"github.com/spf13/cobra"
	"github.com/vito/bass"
)

var Stderr = colorable.NewColorableStderr()

//go:embed txt/help.txt
var helpText string

var rootCmd = &cobra.Command{
	Use:           "bass",
	Short:         "run bass code, or start a repl",
	Long:          helpText,
	Version:       bass.Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.MaximumNArgs(1),
	RunE:          root,
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}

func root(cmd *cobra.Command, args []string) error {
	switch len(args) {
	case 1:
		err := run(bass.New(), args[0])
		if err != nil {
			fmt.Fprintln(Stderr, err)
		}

		return err
	default:
		return repl(bass.New())
	}
}
