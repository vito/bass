package ioctx

import (
	"context"
	"io"
	"io/ioutil"
)

type stdinKey struct{}
type stdoutKey struct{}
type stderrKey struct{}

func StderrFromContext(ctx context.Context) io.Writer {
	logger := ctx.Value(stderrKey{})
	if logger == nil {
		logger = ioutil.Discard
	}

	return logger.(io.Writer)
}

func StderrToContext(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, stderrKey{}, w)
}
