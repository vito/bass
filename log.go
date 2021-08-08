package bass

import (
	"encoding/json"
	"io"
	"time"

	"github.com/mattn/go-colorable"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Logger() *zap.Logger {
	zapcfg := zap.NewDevelopmentEncoderConfig()
	zapcfg.EncodeLevel = zapcore.LowercaseColorLevelEncoder
	zapcfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("15:04:05.000"))
	}

	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zapcfg),
		zapcore.AddSync(colorable.NewColorableStderr()),
		zapcore.DebugLevel,
	))
}

func Dump(dst io.Writer, val interface{}) {
	enc := json.NewEncoder(dst)
	enc.SetIndent("", "  ")
	_ = enc.Encode(val)
}
