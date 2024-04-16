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
	code := m.Run()
	telemetry.Close()
	os.Exit(code)
}
