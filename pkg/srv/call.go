package srv

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
)

type CallHandler struct {
	Cb     bass.Combiner
	RunCtx context.Context
}

func (handler *CallHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// this context intentionally outlives the request so webhooks can be async
	ctx := handler.RunCtx

	// each handler is concurrent, so needs its own trace
	ctx = bass.ForkTrace(ctx)

	request, err := requestToScope(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		cli.WriteError(ctx, err)
		fmt.Fprintf(w, "error: %s\n", err)
		return
	}

	sink := bass.NewSink(bass.NewJSONSink("response", w))

	wg := new(sync.WaitGroup)
	wg.Add(1)

	var responded bool
	respond := bass.Func("respond", "[response]", func(response bass.Value) error {
		err := sink.PipeSink.Emit(response)
		if err != nil {
			return err
		}

		responded = true

		wg.Done()

		return nil
	})

	go func() {
		defer func() {
			if !responded {
				wg.Done()
			}
		}()

		_, err := bass.Trampoline(ctx, handler.Cb.Call(ctx, bass.NewList(request, respond), bass.NewEmptyScope(), bass.Identity))
		if err != nil {
			if !responded {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "error: %s\n", err)
			}

			cli.WriteError(ctx, err)
		}
	}()

	wg.Wait()
}
