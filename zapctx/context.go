package zapctx

import (
	"context"

	"go.uber.org/zap"
)

type logKey struct{}

func FromContext(ctx context.Context) *zap.Logger {
	logger := ctx.Value(logKey{})
	if logger == nil {
		logger = zap.NewNop()
	}

	return logger.(*zap.Logger)
}

func ToContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, logKey{}, logger)
}

func With(ctx context.Context, fields ...zap.Field) (context.Context, *zap.Logger) {
	logger := FromContext(ctx).With(fields...)
	return ToContext(ctx, logger), logger
}
