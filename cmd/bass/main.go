package main

import (
	"context"
	_ "embed"
	"os"
	"os/signal"

	"github.com/mattn/go-colorable"
	"github.com/spf13/cobra"
	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	_ "github.com/vito/bass/runtimes/docker"
	"github.com/vito/bass/zapctx"
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
	logger := bass.Logger()

	ctx := zapctx.ToContext(context.Background(), logger)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	trace := &bass.Trace{}

	ctx = bass.WithTrace(ctx, trace)

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		bass.WriteError(ctx, Stderr, err)
		os.Exit(1)
	}
}

func root(cmd *cobra.Command, argv []string) error {
	ctx := cmd.Context()

	config, err := bass.LoadConfig()
	if err != nil {
		return err
	}

	dispatch, err := runtimes.NewPool(config)
	if err != nil {
		return err
	}

	if len(argv) == 0 {
		return repl(ctx, runtimes.NewEnv(dispatch))
	}

	return run(ctx, runtimes.NewEnv(dispatch), argv[0])
}
