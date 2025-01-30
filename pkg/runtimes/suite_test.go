package runtimes_test

import (
	"context"
	"testing"

	"dagger.io/dagger/telemetry"
	"github.com/vito/bass/pkg/testctx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

type RuntimesSuite struct{}

const InstrumentationLibrary = "bass-lang.org/tests"

func Tracer() trace.Tracer {
	return otel.Tracer(InstrumentationLibrary)
}

func Logger(ctx context.Context) log.Logger {
	return telemetry.Logger(ctx, InstrumentationLibrary)
}

func TestRuntimes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ctx = telemetry.InitEmbedded(ctx, nil)
	t.Cleanup(telemetry.Close)

	testctx.Run(ctx, t, RuntimesSuite{},
		testctx.WithParallel,
		testctx.WithOTelLogging[*testing.T](Logger(ctx)),
		testctx.WithOTelTracing[*testing.T](Tracer()))
}
