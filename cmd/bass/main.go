package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mattn/go-colorable"
	"github.com/spf13/cobra"
	"github.com/vito/bass"
	"github.com/vito/bass/ioctx"
	"github.com/vito/bass/runtimes"
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

var profPort int

func init() {
	rootCmd.Flags().IntVarP(&profPort, "profile", "p", 0, "port number to bind for Go HTTP profiling")
}

func main() {
	logger := bass.Logger()

	ctx := zapctx.ToContext(context.Background(), logger)

	trace := &bass.Trace{}

	ctx = bass.WithTrace(ctx, trace)

	// wire up stderr for (log), (debug), etc.
	ctx = ioctx.StderrToContext(ctx, Stderr)

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		bass.WriteError(ctx, Stderr, err)
		os.Exit(1)
	}
}

var DefaultConfig = bass.Config{
	Runtimes: []bass.RuntimeConfig{
		{
			Platform: bass.Object{
				bass.PlatformOS: bass.String(runtime.GOOS),
			},
			Runtime: runtimes.DockerName,
		},
	},
}

func root(cmd *cobra.Command, argv []string) error {
	ctx := cmd.Context()

	if profPort != 0 {
		zapctx.FromContext(ctx).Sugar().Debugf("serving pprof on :%d", profPort)

		go func() {
			log.Println(http.ListenAndServe(fmt.Sprintf(":%d", profPort), nil))
		}()
	}

	config, err := bass.LoadConfig(DefaultConfig)
	if err != nil {
		return err
	}

	pool, err := runtimes.NewPool(config)
	if err != nil {
		return err
	}

	if runExport {
		return export(ctx, pool)
	}

	state := runtimes.RunState{
		Args:   bass.NewList(),
		Stdin:  bass.Stdin,
		Stdout: bass.Stdout,
	}

	if len(argv) == 0 {
		state.Dir = bass.HostPath{"."}
		return repl(ctx, runtimes.NewEnv(pool, state))
	}

	state.Dir = bass.HostPath{filepath.Dir(argv[0])}
	return run(ctx, runtimes.NewEnv(pool, state), argv[0])
}
