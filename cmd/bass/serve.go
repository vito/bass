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

func serve(ctx context.Context, addr, dir string) error {
	logger := zapctx.FromContext(ctx)

	logger.Info("serving", zap.String("addr", addr), zap.String("dir", dir))

	server := &http.Server{
		Addr: addr,
		Handler: &srv.Handler{
			Dir: dir,
			Env: bass.ImportSystemEnv(),
		},
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	return server.ListenAndServe()
}
