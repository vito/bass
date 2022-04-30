package main

import (
	"context"
	"net"
	"net/http"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/srv"
	"github.com/vito/bass/pkg/zapctx"
	"go.uber.org/zap"
)

// MaxBytes is the maximum size of a request payload.
//
// It is arbitrarily set to 25MB, a value based on GitHub's default payload
// limit.
//
// Bass server servers are not designed to handle unbounded or streaming
// payloads, and sometimes need to buffer the entire request body in order to
// check HMAC signatures, so a reasonable default limit is enforced to help
// prevent DoS attacks.
const MaxBytes = 25 * 1024 * 1024

func serve(ctx context.Context, addr, dir string) error {
	logger := zapctx.FromContext(ctx)

	logger.Info("serving", zap.String("addr", addr), zap.String("dir", dir))

	server := &http.Server{
		Addr: addr,
		Handler: http.MaxBytesHandler(&srv.Handler{
			Dir: dir,
			Env: bass.ImportSystemEnv(),
		}, MaxBytes),
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	return server.ListenAndServe()
}
