package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"

	flag "github.com/spf13/pflag"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/zapctx"
)

var flags = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

var inputs []string

var forwardAddr string
var runExport bool
var bumpLock string
var runPrune bool

var runLSP bool
var lspLogs string

var profPort int
var profFilePath string

var showHelp bool
var showVersion bool

func init() {
	flags.SetOutput(os.Stdout)
	flags.SortFlags = false

	flags.StringSliceVarP(&inputs, "input", "i", nil, "inputs to encode as JSON on *stdin*, name=value; value may be a path")

	flags.BoolVarP(&runExport, "export", "e", false, "write a thunk path to stdout as a tar stream, or log the tar contents if stdout is a tty")
	flags.StringVarP(&bumpLock, "bump", "b", "", "re-generate all values in a bass.lock file")

	flags.BoolVarP(&runPrune, "prune", "p", false, "release data and caches retained by runtimes")

	flags.StringVarP(&forwardAddr, "forward", "f", "", "forward runtimes through a remote SSH server")

	flags.BoolVar(&runLSP, "lsp", false, "run the bass language server")
	flags.StringVar(&lspLogs, "lsp-log-file", "", "write language server logs to this file")

	flags.IntVar(&profPort, "profile", 0, "port number to bind for Go HTTP profiling")
	flags.StringVar(&profFilePath, "cpu-profile", "", "take a CPU profile and save it to this path")

	flags.BoolVarP(&showVersion, "version", "v", false, "print the version number and exit")
	flags.BoolVarP(&showHelp, "help", "h", false, "show bass usage and exit")
}

func main() {
	ctx := zapctx.ToContext(context.Background(), bass.Logger())
	ctx = bass.WithTrace(ctx, &bass.Trace{})
	ctx = ioctx.StderrToContext(ctx, os.Stderr)

	err := flags.Parse(os.Args[1:])
	if err != nil {
		cli.WriteError(ctx, bass.FlagError{
			Err:   err,
			Flags: flags,
		})
		os.Exit(2)
		return
	}

	err = root(ctx)
	if err != nil {
		os.Exit(1)
	}
}

var DefaultConfig = bass.Config{
	Runtimes: []bass.RuntimeConfig{
		{
			Platform: bass.LinuxPlatform,
			Runtime:  runtimes.BuildkitName,
			Addrs:    runtimes.DefaultBuildkitAddrs,
		},
	},
}

func root(ctx context.Context) error {
	if showVersion {
		printVersion(ctx)
		return nil
	}

	if showHelp {
		help(ctx)
		return nil
	}

	if profPort != 0 {
		zapctx.FromContext(ctx).Sugar().Debugf("serving pprof on :%d", profPort)

		l, err := net.Listen("tcp", fmt.Sprintf(":%d", profPort))
		if err != nil {
			cli.WriteError(ctx, err)
			return err
		}

		go http.Serve(l, nil)
	}

	if profFilePath != "" {
		profFile, err := os.Create(profFilePath)
		if err != nil {
			cli.WriteError(ctx, err)
			return err
		}

		defer profFile.Close()

		pprof.StartCPUProfile(profFile)
		defer pprof.StopCPUProfile()
	}

	config, err := bass.LoadConfig(DefaultConfig)
	if err != nil {
		cli.WriteError(ctx, err)
		return err
	}

	pool, err := runtimes.NewPool(config)
	if err != nil {
		cli.WriteError(ctx, err)
		return err
	}

	ctx = bass.WithRuntimePool(ctx, pool)

	if forwardAddr != "" {
		return forwardLoop(ctx, forwardAddr, config.Runtimes)
	}

	if runExport {
		return export(ctx)
	}

	if runPrune {
		return prune(ctx)
	}

	if runLSP {
		return langServer(ctx)
	}

	if bumpLock != "" {
		return bump(ctx)
	}

	argv := flags.Args()

	if len(argv) == 0 {
		return repl(ctx)
	}

	return run(ctx, argv[0], argv[1:]...)
}

func repl(ctx context.Context) error {
	env := bass.ImportSystemEnv()

	scope := bass.NewRunScope(bass.Ground, bass.RunState{
		Dir:    bass.NewHostDir("."),
		Stdin:  bass.Stdin,
		Stdout: bass.Stdout,
		Env:    env,
	})

	return cli.Repl(ctx, scope)
}
