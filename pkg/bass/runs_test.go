package bass_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
)

func TestRuns(t *testing.T) {
	is := is.New(t)

	ctx := context.Background()
	ctx, runs := bass.TrackRuns(ctx)

	errorScpt := bass.NewInMemoryFile("error.bass", `
		(error "oh no")
	`)

	thunk := bass.Thunk{
		Cmd: bass.ThunkCmd{
			FS: errorScpt,
		},
	}

	comb, err := thunk.Start(ctx, bass.Func("done", "[ok?]", func(ok bool) error {
		if !ok {
			return fmt.Errorf("it failed!")
		}

		return nil
	}))

	_, err = basstest.Call(comb, bass.NewEmptyScope(), bass.NewList())
	is.True(err != nil)
	is.Equal(err.Error(), "it failed!: oh no")

	err = runs.Wait()
	is.True(err != nil)
	is.Equal(err.Error(), "it failed!: oh no")
}
