package main

import (
	"context"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/progrock"
)

func run(ctx context.Context) error {
	ctx, _, err := setupPool(ctx, true)
	if err != nil {
		return err
	}

	return cli.Step(ctx, cmdline, func(ctx context.Context, vtx *progrock.VertexRecorder) error {
		isTty := isatty.IsTerminal(os.Stdout.Fd())

		stdout := bass.Stdout
		if isTty {
			stdout = bass.NewSink(bass.NewJSONSink("stdout vertex", vtx.Stdout()))
		}

		argv := flags.Args()

		err := cli.Run(ctx, bass.ImportSystemEnv(), inputs, argv[0], argv[1:], stdout)

		if !isTty {
			// ensure a chained unix pipeline exits
			os.Stdout.Close()
		}

		return err
	})
}
