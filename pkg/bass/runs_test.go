package bass_test

import (
	"context"
	"fmt"
	"strings"
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
		(defn main [msg]
			(error msg))
	`)

	thunk1 := bass.MustThunk(errorScpt).AppendArgs(bass.String("oh no"))
	thunk2 := bass.MustThunk(errorScpt).AppendArgs(bass.String("let's go"))

	errCb := bass.Func("err-if-not-ok", "[err]", func(merr bass.Value) error {
		var errv bass.Error
		if err := merr.Decode(&errv); err == nil {
			return fmt.Errorf("it failed!: %w", errv.Err)
		}

		return nil
	})

	comb1, err := thunk1.Start(ctx, errCb)
	is.NoErr(err)

	comb2, err := thunk2.Start(ctx, errCb)
	is.NoErr(err)

	_, err = basstest.Call(comb1, bass.NewEmptyScope(), bass.NewList())
	is.True(err != nil)
	is.Equal(err.Error(), "it failed!: oh no")

	_, err = basstest.Call(comb2, bass.NewEmptyScope(), bass.NewList())
	is.True(err != nil)
	is.Equal(err.Error(), "it failed!: let's go")

	err = runs.Wait()
	is.True(err != nil)
	is.True(strings.Contains(err.Error(), "it failed!: oh no"))
	is.True(strings.Contains(err.Error(), "it failed!: let's go"))
}
