package bass

import (
	"context"
	"sync"

	"github.com/hashicorp/go-multierror"
)

type Runs struct {
	wg sync.WaitGroup

	errs  error
	errsL sync.Mutex
}

func (runs *Runs) Go(f func() error) {
	runs.wg.Add(1)
	go func() {
		defer runs.wg.Done()
		runs.record(f())
	}()
}

func (runs *Runs) Wait() error {
	runs.wg.Wait()
	return runs.errs
}

func (runs *Runs) record(err error) {
	runs.errsL.Lock()
	if runs.errs != nil {
		runs.errs = multierror.Append(runs.errs, err)
	} else {
		runs.errs = err
	}
	runs.errsL.Unlock()
}

type runsKey struct{}

func TrackRuns(ctx context.Context) (context.Context, *Runs) {
	runs := new(Runs)
	return context.WithValue(ctx, runsKey{}, runs), runs
}

func RunsFromContext(ctx context.Context) *Runs {
	runs := ctx.Value(runsKey{})
	if runs != nil {
		return runs.(*Runs)
	}

	return new(Runs)
}
