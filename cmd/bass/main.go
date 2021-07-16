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
	"github.com/vito/bass/cmd/bass/cli"
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
	RunE:          root,
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		fmt.Fprintln(Stderr, err)
		os.Exit(1)
	}
}

func root(cmd *cobra.Command, argv []string) error {
	if len(argv) == 0 {
		env := bass.NewRuntimeEnv(bass.RuntimeState{
			Stderr: bass.Stderr,
		})

		return repl(env)
	}

	file, args, err := cli.ParseArgs(argv)
	if err != nil {
		return err
	}

	env := bass.NewRuntimeEnv(bass.RuntimeState{
		Stderr: bass.Stderr,
		Args:   args,
	})

	return run(env, file)
}
