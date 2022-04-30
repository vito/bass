package srv

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/zapctx"
	"go.uber.org/zap"
)

type Handler struct {
	Dir string
	Env *bass.Scope
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logger := zapctx.FromContext(ctx)

	if r.URL.Path == "/favicon.ico" {
		logger.Info("ignoring favicon")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	r.Write(os.Stderr)

	logger.Info("serving", zap.String("path", r.URL.Path))

	script := filepath.Join(handler.Dir, filepath.FromSlash(path.Clean(r.URL.Path)))

	scope := runtimes.NewScope(bass.Ground, runtimes.RunState{
		Dir:    bass.NewHostDir(filepath.Dir(script)),
		Env:    handler.Env.Copy(), // NB: sharing this should be carefully considered
		Stdin:  bass.NewSource(bass.NewJSONSource("request", r.Body)),
		Stdout: bass.NewSink(bass.NewJSONSink("response", w)),
	})

	_, err := bass.EvalFile(ctx, scope, script)
	if err != nil {
		logger.Error("errored loading script", zap.Error(err))
		// TODO: this will fail if a response is already written - do we need
		// something that can handle results and then an error?
		w.WriteHeader(http.StatusInternalServerError)
		cli.WriteError(ctx, err)
		fmt.Fprintf(w, "error: %s\n", err)
		return
	}

	err = bass.RunMain(ctx, scope)
	if err != nil {
		logger.Error("errored running main", zap.Error(err))
		// TODO: this will fail if a response is already written - do we need
		// something that can handle results and then an error?
		w.WriteHeader(http.StatusInternalServerError)
		cli.WriteError(ctx, err)
		fmt.Fprintf(w, "error: %s\n", err)
		return
	}

	logger.Debug("served", zap.String("path", r.URL.Path))
}
