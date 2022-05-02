package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/srv"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/progrock"
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
	var err error
	if dir == "" {
		dir, err = os.Getwd()
	} else {
		dir, err = filepath.Abs(dir)
	}
	if err != nil {
		return err
	}

	return withProgress(ctx, "serve", func(ctx context.Context, vertex *progrock.VertexRecorder) error {
		logger := zapctx.FromContext(ctx)

		logger.Info("listening", zap.String("addr", addr), zap.String("dir", dir))

		server := &http.Server{
			Addr: addr,
			Handler: http.MaxBytesHandler(&srv.Handler{
				Dir:    dir,
				Env:    bass.ImportSystemEnv(),
				RunCtx: ctx,
			}, MaxBytes),
		}

		go func() {
			<-ctx.Done()
			// just passing ctx along to immediately interrupt everything
			server.Shutdown(ctx)
		}()

		return server.ListenAndServe()
	})
}
