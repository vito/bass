package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"
	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/bass/zapctx"
)

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
var profFilePath string

func init() {
	rootCmd.Flags().IntVarP(&profPort, "profile", "p", 0, "port number to bind for Go HTTP profiling")
	rootCmd.Flags().StringVar(&profFilePath, "cpu-profile", "", "file to dump CPU profile to")
}

func main() {
	logger := bass.Logger()
	ctx := zapctx.ToContext(context.Background(), logger)

	trace := &bass.Trace{}
	ctx = bass.WithTrace(ctx, trace)

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}

var DefaultConfig = bass.Config{
	Runtimes: []bass.RuntimeConfig{
		{
			Platform: bass.LinuxPlatform,
			Runtime:  runtimes.DockerName,
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

	if profFilePath != "" {
		profFile, err := os.Create(profFilePath)
		if err != nil {
			return err
		}

		defer profFile.Close()

		pprof.StartCPUProfile(profFile)
		defer pprof.StopCPUProfile()
	}

	config, err := bass.LoadConfig(DefaultConfig)
	if err != nil {
		return err
	}

	pool, err := runtimes.NewPool(config)
	if err != nil {
		return err
	}

	ctx = bass.WithRuntime(ctx, pool)

	if runExport {
		return export(ctx, pool)
	}

	if len(argv) == 0 {
		return repl(ctx)
	}

	return run(ctx, argv[0], argv[1:]...)
}
