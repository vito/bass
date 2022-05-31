package bass

import (
	"context"
	"sync"
)

type Runs struct {
	sync.WaitGroup
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
