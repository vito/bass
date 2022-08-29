package main

import (
	"io"
	"net"
	"time"

	"github.com/mattn/go-colorable"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// yoinked from pkg/bass/log.go, avoiding too many dependencies
func LoggerTo(w io.Writer, level zapcore.LevelEnabler) *zap.Logger {
	zapcfg := zap.NewDevelopmentEncoderConfig()
	zapcfg.EncodeLevel = zapcore.LowercaseColorLevelEncoder
	zapcfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("15:04:05.000"))
	}

	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zapcfg),
		zapcore.AddSync(w),
		level,
	))
}

func StdLogger(level zapcore.LevelEnabler) *zap.Logger {
	return LoggerTo(colorable.NewColorableStderr(), level)
}

func logIPs(logger *zap.Logger) {
	ifaces, err := net.Interfaces()
	if err != nil {
		logger.Error("failed to get interfaces", zap.Error(err))
		return
	}

	for _, i := range ifaces {
		logger = logger.With(zap.String("iface", i.Name))

		addrs, err := i.Addrs()
		if err != nil {
			logger.Error("failed to get addrs", zap.Error(err))
			continue
		}

		for _, addr := range addrs {
			logger.Debug("addr", zap.String("addr", addr.String()))
		}
	}
}
