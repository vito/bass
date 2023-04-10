package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"
	"strings"

	flag "github.com/spf13/pflag"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/zapctx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var flags = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
var cmdline = strings.Join(os.Args, " ")

var inputs []string

var runRun bool
var runExport bool
var runBump bool
var runPrune bool
var runnerAddr string

var runLSP bool
var lspLogs string

var profPort int
var profFilePath string

var showHelp bool
var showVersion bool
var showDebug bool

func init() {
	flags.SetOutput(os.Stdout)
	flags.SortFlags = false

	flags.StringSliceVarP(&inputs, "input", "i", nil, "inputs to encode as JSON on *stdin*, name=value; value may be a path")

	flags.BoolVarP(&runExport, "export", "e", false, "write a thunk path to stdout as a tar stream, or log the tar contents if stdout is a tty")
	flags.BoolVar(&runRun, "run", false, "run a thunk read from stdin in JSON format")
	flags.BoolVarP(&runBump, "bump", "b", false, "re-generate all calls in bass.lock files")

	flags.BoolVarP(&runPrune, "prune", "p", false, "release data and caches retained by runtimes")

	flags.StringVarP(&runnerAddr, "runner", "r", "", "serve locally configured runtimes over SSH")

	flags.BoolVar(&runLSP, "lsp", false, "run the bass language server")
	flags.StringVar(&lspLogs, "lsp-log-file", "", "write language server logs to this file")

	flags.IntVar(&profPort, "profile", 0, "port number to bind for Go HTTP profiling")
	flags.StringVar(&profFilePath, "cpu-profile", "", "take a CPU profile and save it to this path")

	flags.BoolVarP(&showVersion, "version", "v", false, "print the version number and exit")
	flags.BoolVarP(&showHelp, "help", "h", false, "show bass usage and exit")

	flags.BoolVar(&showDebug, "debug", false, "show debug logs")
}

func logLevel() zapcore.LevelEnabler {
	if showDebug {
		return zap.DebugLevel
	} else {
		return zap.InfoLevel
	}
}

func main() {
	ctx := context.Background()
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

	ctx = zapctx.ToContext(ctx, bass.StdLogger(logLevel()))

	err = root(ctx)
	if err != nil {
		os.Exit(1)
	}
}

var DefaultConfig = bass.Config{
	Runtimes: []bass.RuntimeConfig{},
}

func init() {
	var runtime string
	if os.Getenv("DAGGER_SESSION_PORT") != "" || os.Getenv("_EXPERIMENTAL_DAGGER_CLI_BIN") != "" {
		runtime = runtimes.DaggerName
	} else {
		runtime = runtimes.BuildkitName
	}

	DefaultConfig.Runtimes = []bass.RuntimeConfig{
		{
			Platform: bass.LinuxPlatform,
			Runtime:  runtime,
		},
	}
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

	pool, err := runtimes.NewPool(ctx, config)
	if err != nil {
		cli.WriteError(ctx, err)
		return err
	}

	ctx = bass.WithRuntimePool(ctx, pool)

	if runnerAddr != "" {
		client, err := runnerClient(ctx, runnerAddr)
		if err != nil {
			cli.WriteError(ctx, err)
			return err
		}

		return cli.WithProgress(ctx, func(ctx context.Context) error {
			return runnerLoop(ctx, client, pool.Runtimes)
		})
	}

	if runExport {
		return cli.WithProgress(ctx, export)
	}

	if runPrune {
		return cli.WithProgress(ctx, prune)
	}

	if runLSP {
		return langServer(ctx)
	}

	if runBump {
		return cli.WithProgress(ctx, bump)
	}

	if runRun {
		return cli.WithProgress(ctx, runThunk)
	}

	if flags.NArg() == 0 {
		return repl(ctx)
	}

	return cli.WithProgress(ctx, run)
}

func repl(ctx context.Context) error {
	scope := bass.NewRunScope(bass.Ground, bass.RunState{
		Dir:    bass.NewHostDir("."),
		Stdin:  bass.Stdin,
		Stdout: bass.Stdout,
		Env:    bass.ImportSystemEnv(),
	})

	return cli.Repl(ctx, scope)
}
