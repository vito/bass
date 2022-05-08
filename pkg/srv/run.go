package srv

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"path/filepath"

	"github.com/opencontainers/go-digest"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/progrock"
	"go.uber.org/zap"
)

type RunHandler struct {
	Dir    string
	Env    *bass.Scope
	RunCtx context.Context
}

func (handler *RunHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// this context intentionally outlives the request so webhooks can be async
	ctx := handler.RunCtx

	// each handler is concurrent, so needs its own trace
	ctx = bass.ForkTrace(ctx)

	script := filepath.Join(handler.Dir, filepath.FromSlash(path.Clean(r.URL.Path)))

	request, err := requestToScope(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		cli.WriteError(ctx, err)
		fmt.Fprintf(w, "error: %s\n", err)
		return
	}

	dir := filepath.Dir(script)
	scope := bass.NewRunScope(bass.Ground, bass.RunState{
		Dir:    bass.NewHostDir(dir),
		Env:    bass.NewEmptyScope(handler.Env),
		Stdin:  bass.NewSource(bass.NewInMemorySource(request)),
		Stdout: bass.NewSink(bass.NewJSONSink("response", w)),
	})

	analogousThunk := bass.Thunk{
		Cmd: bass.ThunkCmd{
			Host: &bass.HostPath{
				ContextDir: dir,
				Path: bass.FileOrDirPath{
					File: &bass.FilePath{Path: filepath.Base(script)},
				},
			},
		},
		Stdin: []bass.Value{request},
	}

	name := analogousThunk.Name()
	recorder := progrock.RecorderFromContext(ctx)
	bassVertex := recorder.Vertex(digest.Digest(name), fmt.Sprintf("bass %s", analogousThunk.Cmdline()))
	defer func() { bassVertex.Done(err) }()

	stderr := bassVertex.Stderr()

	// wire up logs to vertex
	logger := bass.LoggerTo(stderr)
	ctx = zapctx.ToContext(ctx, logger)

	// wire up stderr for (log), (debug), etc.
	ctx = ioctx.StderrToContext(ctx, stderr)

	_, err = bass.EvalFile(ctx, scope, script)
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
}
