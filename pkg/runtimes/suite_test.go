package runtimes_test

import (
	"os"
	"testing"

	"github.com/dagger/testctx"
	"github.com/dagger/testctx/oteltest"
)

type RuntimesSuite struct{}

func TestMain(m *testing.M) {
	os.Exit(oteltest.Main(m))
}

func TestRuntimes(t *testing.T) {
	testctx.New(t,
		testctx.WithParallel(),
		oteltest.WithTracing[*testing.T](),
		oteltest.WithLogging[*testing.T]()).
		RunTests(RuntimesSuite{})
}
