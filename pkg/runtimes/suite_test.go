package runtimes_test

import (
	"context"
	"os"
	"testing"

	"dagger.io/dagger/telemetry"
)

var testCtx = context.Background()

func TestMain(m *testing.M) {
	testCtx = telemetry.InitEmbedded(testCtx, nil)
	exitCode := m.Run()
	telemetry.Close()
	os.Exit(exitCode)
}
